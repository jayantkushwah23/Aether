package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

// Mint issues factory coins according to the denom's configured economic supply mode.
func (ms msgServer) Mint(goCtx context.Context, msg *types.MsgMint) (*types.MsgMintResponse, error) {
	if msg == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil MsgMint")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	admin, err := sdk.AccAddressFromBech32(msg.Admin)
	if err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
	}
	if err := sdk.ValidateDenom(msg.Denom); err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}
	if msg.Amount.IsNil() || !msg.Amount.IsPositive() {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidCoins, "mint amount must be positive")
	}

	recipient := admin
	if msg.Recipient != "" {
		recipient, err = sdk.AccAddressFromBech32(msg.Recipient)
		if err != nil {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
		}
	}

	policy, err := ms.GetDenomPolicy(ctx, msg.Denom)
	if err != nil {
		return nil, err
	}

	switch policy.SupplyMode {
	case types.SUPPLY_MODE_CAPPED:
		return ms.mintCapped(ctx, msg, admin, recipient, policy)
	case types.SUPPLY_MODE_UNCAPPED:
		return ms.mintUncapped(ctx, msg, admin, recipient, policy)
	case types.SUPPLY_MODE_COLLATERALIZED:
		return ms.mintCollateralized(ctx, msg, admin, recipient, policy)
	default:
		return nil, errorsmod.Wrapf(types.ErrInvalidSupplyMode, "unsupported supply mode %d", policy.SupplyMode)
	}
}

// Burn handles destruction of tenant tokens according to the denom's configured economic supply mode.
func (ms msgServer) Burn(goCtx context.Context, msg *types.MsgBurn) (*types.MsgBurnResponse, error) {
	if msg == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil MsgBurn")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
	}
	if err := sdk.ValidateDenom(msg.Denom); err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}
	if msg.Amount.IsNil() || !msg.Amount.IsPositive() {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidCoins, "burn amount must be positive")
	}

	policy, err := ms.GetDenomPolicy(ctx, msg.Denom)
	if err != nil {
		return nil, err
	}

	switch policy.SupplyMode {
	case types.SUPPLY_MODE_CAPPED:
		return ms.burnCapped(ctx, msg, sender, policy)
	case types.SUPPLY_MODE_UNCAPPED:
		return ms.burnUncapped(ctx, msg, sender, policy)
	case types.SUPPLY_MODE_COLLATERALIZED:
		return ms.burnCollateralized(ctx, msg, sender, policy)
	default:
		return nil, errorsmod.Wrapf(types.ErrInvalidSupplyMode, "unsupported supply mode %d", policy.SupplyMode)
	}
}

// -----------------------------------------------------------------------------
// SUPPLY_MODE_CAPPED Handlers
// -----------------------------------------------------------------------------

func (ms msgServer) mintCapped(
	ctx sdk.Context,
	msg *types.MsgMint,
	admin, recipient sdk.AccAddress,
	policy types.DenomPolicy,
) (*types.MsgMintResponse, error) {
	policyAdmin, err := sdk.AccAddressFromBech32(policy.Admin)
	if err != nil {
		return nil, err
	}
	if !admin.Equals(policyAdmin) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not the admin of %s", admin, msg.Denom)
	}

	if policy.MaxSupply.IsNil() || !policy.MaxSupply.IsPositive() {
		return nil, errorsmod.Wrapf(types.ErrSupplyCapExceeded, "capped denom %s has no max_supply", msg.Denom)
	}

	// 1. If can_burn is true: Query bank keeper current supply; reject if current_supply + mint_amount > max_supply.
	// 2. If can_burn is false: Track cumulative minted_lifetime in a separate collections register and check against that instead.
	var recordedLifetime math.Int

	if policy.CanBurn {
		currentSupply := ms.bankKeeper.GetSupply(ctx, msg.Denom).Amount
		if currentSupply.IsNil() {
			currentSupply = math.ZeroInt()
		}
		newSupply, err := currentSupply.SafeAdd(msg.Amount)
		if err != nil {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
		}
		if newSupply.GT(policy.MaxSupply) {
			return nil, errorsmod.Wrapf(
				types.ErrSupplyCapExceeded,
				"mint amount %s would bring current supply %s above max_supply %s",
				msg.Amount, newSupply, policy.MaxSupply,
			)
		}

		// Also update lifetime register for transparency
		lifetime, _ := ms.GetMintedLifetime(ctx, msg.Denom)
		recordedLifetime = lifetime.Add(msg.Amount)
		_ = ms.SetMintedLifetime(ctx, msg.Denom, recordedLifetime)
	} else {
		lifetime, err := ms.GetMintedLifetime(ctx, msg.Denom)
		if err != nil {
			return nil, err
		}
		newLifetime, err := lifetime.SafeAdd(msg.Amount)
		if err != nil {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
		}
		if newLifetime.GT(policy.MaxSupply) {
			return nil, errorsmod.Wrapf(
				types.ErrSupplyCapExceeded,
				"denom %s minted_lifetime %s exceeds max_supply %s",
				msg.Denom, newLifetime, policy.MaxSupply,
			)
		}

		if err := ms.SetMintedLifetime(ctx, msg.Denom, newLifetime); err != nil {
			return nil, err
		}
		recordedLifetime = newLifetime
	}

	coins := sdk.NewCoins(sdk.NewCoin(msg.Denom, msg.Amount))
	if err := ms.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coins); err != nil {
		return nil, err
	}

	policy.MintedLifetime = recordedLifetime
	_ = ms.SetDenomPolicy(ctx, policy)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mint_denom",
		sdk.NewAttribute("denom", msg.Denom),
		sdk.NewAttribute("admin", admin.String()),
		sdk.NewAttribute("recipient", recipient.String()),
		sdk.NewAttribute("amount", msg.Amount.String()),
		sdk.NewAttribute("supply_mode", "capped"),
		sdk.NewAttribute("minted_lifetime", recordedLifetime.String()),
	))

	return &types.MsgMintResponse{
		Denom:          msg.Denom,
		MintedLifetime: recordedLifetime.String(),
	}, nil
}

func (ms msgServer) burnCapped(
	ctx sdk.Context,
	msg *types.MsgBurn,
	sender sdk.AccAddress,
	policy types.DenomPolicy,
) (*types.MsgBurnResponse, error) {
	if !policy.CanBurn {
		return nil, errorsmod.Wrapf(types.ErrBurnDisabled, "cannot burn tokens for capped denom %s when can_burn is false", msg.Denom)
	}

	coins := sdk.NewCoins(sdk.NewCoin(msg.Denom, msg.Amount))
	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, coins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.BurnCoins(ctx, types.ModuleName, coins); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"burn_denom",
		sdk.NewAttribute("denom", msg.Denom),
		sdk.NewAttribute("sender", sender.String()),
		sdk.NewAttribute("amount", msg.Amount.String()),
		sdk.NewAttribute("supply_mode", "capped"),
	))

	return &types.MsgBurnResponse{
		Denom:  msg.Denom,
		Burned: msg.Amount.String(),
	}, nil
}

// -----------------------------------------------------------------------------
// SUPPLY_MODE_UNCAPPED Handlers
// -----------------------------------------------------------------------------

func (ms msgServer) mintUncapped(
	ctx sdk.Context,
	msg *types.MsgMint,
	admin, recipient sdk.AccAddress,
	policy types.DenomPolicy,
) (*types.MsgMintResponse, error) {
	policyAdmin, err := sdk.AccAddressFromBech32(policy.Admin)
	if err != nil {
		return nil, err
	}
	// Restrict execution strictly to the designated token admin address
	if !admin.Equals(policyAdmin) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not the admin of %s", admin, msg.Denom)
	}

	coins := sdk.NewCoins(sdk.NewCoin(msg.Denom, msg.Amount))
	if err := ms.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return nil, err
	}
	// Allow standard minting directly into their account balance (or specified recipient)
	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coins); err != nil {
		return nil, err
	}

	lifetime, _ := ms.GetMintedLifetime(ctx, msg.Denom)
	newLifetime := lifetime.Add(msg.Amount)
	_ = ms.SetMintedLifetime(ctx, msg.Denom, newLifetime)

	policy.MintedLifetime = newLifetime
	_ = ms.SetDenomPolicy(ctx, policy)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mint_denom",
		sdk.NewAttribute("denom", msg.Denom),
		sdk.NewAttribute("admin", admin.String()),
		sdk.NewAttribute("recipient", recipient.String()),
		sdk.NewAttribute("amount", msg.Amount.String()),
		sdk.NewAttribute("supply_mode", "uncapped"),
		sdk.NewAttribute("minted_lifetime", newLifetime.String()),
	))

	return &types.MsgMintResponse{
		Denom:          msg.Denom,
		MintedLifetime: newLifetime.String(),
	}, nil
}

func (ms msgServer) burnUncapped(
	ctx sdk.Context,
	msg *types.MsgBurn,
	sender sdk.AccAddress,
	policy types.DenomPolicy,
) (*types.MsgBurnResponse, error) {
	policyAdmin, err := sdk.AccAddressFromBech32(policy.Admin)
	if err != nil {
		return nil, err
	}
	// Restrict execution strictly to the designated token admin address
	if !sender.Equals(policyAdmin) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not the admin of %s", sender, msg.Denom)
	}

	coins := sdk.NewCoins(sdk.NewCoin(msg.Denom, msg.Amount))
	// Allow standard burning directly from their account balance
	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, coins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.BurnCoins(ctx, types.ModuleName, coins); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"burn_denom",
		sdk.NewAttribute("denom", msg.Denom),
		sdk.NewAttribute("sender", sender.String()),
		sdk.NewAttribute("amount", msg.Amount.String()),
		sdk.NewAttribute("supply_mode", "uncapped"),
	))

	return &types.MsgBurnResponse{
		Denom:  msg.Denom,
		Burned: msg.Amount.String(),
	}, nil
}

// -----------------------------------------------------------------------------
// SUPPLY_MODE_COLLATERALIZED Handlers
// -----------------------------------------------------------------------------

func (ms msgServer) mintCollateralized(
	ctx sdk.Context,
	msg *types.MsgMint,
	user, recipient sdk.AccAddress,
	policy types.DenomPolicy,
) (*types.MsgMintResponse, error) {
	// Read the configured collateral_denom and collateral_ratio
	collateralDenom := policy.CollateralDenom
	if collateralDenom == "" {
		return nil, errorsmod.Wrapf(types.ErrInvalidCollateral, "collateral_denom is not configured for %s", msg.Denom)
	}

	ratio := policy.CollateralRatio
	if ratio.IsNil() || !ratio.IsPositive() {
		ratio = math.LegacyOneDec()
	}

	// Calculate required collateral: ceil(amount * collateral_ratio)
	requiredCollateral := msg.Amount.ToLegacyDec().Mul(ratio).Ceil().TruncateInt()
	if !requiredCollateral.IsPositive() {
		return nil, errorsmod.Wrap(types.ErrInvalidCollateral, "calculated required collateral must be positive")
	}

	collateralCoins := sdk.NewCoins(sdk.NewCoin(collateralDenom, requiredCollateral))

	// Programmatically transfer those collateral assets from the user's wallet into the secure currencies module escrow account
	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, user, types.ModuleName, collateralCoins); err != nil {
		return nil, errorsmod.Wrapf(types.ErrInsufficientCollateral, "failed to transfer required collateral %s: %s", collateralCoins, err.Error())
	}

	// Mint the corresponding quantity of the tenant token directly to the user (or recipient)
	tenantCoins := sdk.NewCoins(sdk.NewCoin(msg.Denom, msg.Amount))
	if err := ms.bankKeeper.MintCoins(ctx, types.ModuleName, tenantCoins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, tenantCoins); err != nil {
		return nil, err
	}

	lifetime, _ := ms.GetMintedLifetime(ctx, msg.Denom)
	newLifetime := lifetime.Add(msg.Amount)
	_ = ms.SetMintedLifetime(ctx, msg.Denom, newLifetime)

	policy.MintedLifetime = newLifetime
	_ = ms.SetDenomPolicy(ctx, policy)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mint_denom",
		sdk.NewAttribute("denom", msg.Denom),
		sdk.NewAttribute("user", user.String()),
		sdk.NewAttribute("recipient", recipient.String()),
		sdk.NewAttribute("amount", msg.Amount.String()),
		sdk.NewAttribute("supply_mode", "collateralized"),
		sdk.NewAttribute("collateral_denom", collateralDenom),
		sdk.NewAttribute("collateral_escrowed", requiredCollateral.String()),
		sdk.NewAttribute("collateral_ratio", ratio.String()),
	))

	return &types.MsgMintResponse{
		Denom:          msg.Denom,
		MintedLifetime: newLifetime.String(),
	}, nil
}

func (ms msgServer) burnCollateralized(
	ctx sdk.Context,
	msg *types.MsgBurn,
	user sdk.AccAddress,
	policy types.DenomPolicy,
) (*types.MsgBurnResponse, error) {
	// Transfer tenant coins from user's account to module and burn them
	tenantCoins := sdk.NewCoins(sdk.NewCoin(msg.Denom, msg.Amount))
	if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, user, types.ModuleName, tenantCoins); err != nil {
		return nil, err
	}
	if err := ms.bankKeeper.BurnCoins(ctx, types.ModuleName, tenantCoins); err != nil {
		return nil, err
	}

	// Calculate and release escrowed collateral back to the user: floor(amount * collateral_ratio)
	ratio := policy.CollateralRatio
	if ratio.IsNil() || !ratio.IsPositive() {
		ratio = math.LegacyOneDec()
	}

	collateralToRefund := msg.Amount.ToLegacyDec().Mul(ratio).TruncateInt()
	if collateralToRefund.IsPositive() && policy.CollateralDenom != "" {
		collateralCoins := sdk.NewCoins(sdk.NewCoin(policy.CollateralDenom, collateralToRefund))
		if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, user, collateralCoins); err != nil {
			return nil, fmt.Errorf("failed to release escrowed collateral %s: %w", collateralCoins, err)
		}
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"burn_denom",
		sdk.NewAttribute("denom", msg.Denom),
		sdk.NewAttribute("user", user.String()),
		sdk.NewAttribute("amount", msg.Amount.String()),
		sdk.NewAttribute("supply_mode", "collateralized"),
		sdk.NewAttribute("collateral_denom", policy.CollateralDenom),
		sdk.NewAttribute("collateral_refunded", collateralToRefund.String()),
	))

	return &types.MsgBurnResponse{
		Denom:  msg.Denom,
		Burned: msg.Amount.String(),
	}, nil
}
