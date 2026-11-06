package keeper

import (
	"context"
	"fmt"
	"strings"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

type msgServer struct {
	Keeper
}

var _ types.MsgServer = msgServer{}

// NewMsgServerImpl returns the currencies message server.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

// CreateDenom validates and deploys a new factory denom and its DenomPolicy.
func (ms msgServer) CreateDenom(goCtx context.Context, msg *types.MsgCreateDenom) (*types.MsgCreateDenomResponse, error) {
	if msg == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil MsgCreateDenom")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate creator address
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
	}

	// 1. Verify the subdenom consists only of alphanumeric characters and forms a valid canonical denom string: factory/{creator}/{subdenom}
	if err := types.ValidateAlphanumericSubdenom(msg.Subdenom); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidSubdenom, err.Error())
	}

	denom := types.FormatDenom(creator.String(), msg.Subdenom)
	if err := sdk.ValidateDenom(denom); err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid canonical denom %s: %s", denom, err.Error())
	}

	// 2. Check the CreatorIndex using cosmossdk.io/collections to ensure this creator account hasn't already deployed this exact subdenom
	alreadyDeployed, err := ms.HasCreatorSubdenom(ctx, creator.String(), msg.Subdenom)
	if err != nil {
		return nil, err
	}
	if alreadyDeployed {
		return nil, errorsmod.Wrapf(types.ErrDenomExists, "creator %s already deployed subdenom %s", creator.String(), msg.Subdenom)
	}

	// Defense-in-depth: Ensure denom policy doesn't already exist
	exists, err := ms.HasDenomPolicy(ctx, denom)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errorsmod.Wrapf(types.ErrDenomExists, "denom %s already exists", denom)
	}

	// Validate symbol and supply mode parameters
	if msg.Symbol == "" || len(msg.Symbol) > 32 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "symbol must be 1-32 characters")
	}

	maxSupply := msg.MaxSupply
	switch msg.SupplyMode {
	case types.SUPPLY_MODE_CAPPED:
		if maxSupply.IsNil() || !maxSupply.IsPositive() {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "capped denoms require a positive max_supply")
		}
	case types.SUPPLY_MODE_UNCAPPED, types.SUPPLY_MODE_COLLATERALIZED:
		if !maxSupply.IsNil() && !maxSupply.IsZero() {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "max_supply is only set for capped denoms")
		}
		maxSupply = math.ZeroInt()
	default:
		return nil, errorsmod.Wrapf(types.ErrInvalidSupplyMode, "unsupported supply mode %d", msg.SupplyMode)
	}

	// Enforce transfer fee cap (max 1000 bps = 10%) to prevent complete balance seizures
	if msg.TransferFeeBps > types.MaxTransferFeeBps {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "transfer_fee_bps %d exceeds maximum allowed limit %d (10%%)", msg.TransferFeeBps, types.MaxTransferFeeBps)
	}

	feeCollector := msg.FeeCollectorAddress
	if feeCollector == "" {
		feeCollector = creator.String()
	} else {
		if _, err := sdk.AccAddressFromBech32(feeCollector); err != nil {
			return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid fee_collector_address")
		}
	}

	// 5. Enforce high fixed gas burn as long-term state rent rather than a cheap config write
	ctx.GasMeter().ConsumeGas(types.DefaultCreateDenomStateRentGas, "create_denom_state_rent")

	collateralRatio := msg.CollateralRatio
	if msg.SupplyMode == types.SUPPLY_MODE_COLLATERALIZED {
		if collateralRatio.IsNil() || !collateralRatio.IsPositive() {
			collateralRatio = math.LegacyOneDec()
		}
	} else if collateralRatio.IsNil() {
		collateralRatio = math.LegacyZeroDec()
	}

	// 3. Save the DenomPolicy state struct
	policy := types.DenomPolicy{
		Denom:               denom,
		Admin:               creator.String(),
		Symbol:              msg.Symbol,
		SupplyMode:          msg.SupplyMode,
		MaxSupply:           maxSupply,
		MintedLifetime:      math.ZeroInt(),
		TransferFeeBps:      msg.TransferFeeBps,
		CanBurn:             msg.CanBurn,
		CollateralDenom:     msg.CollateralDenom,
		CollateralRatio:     collateralRatio,
		IsPaused:            false,
		FeeCollectorAddress: feeCollector,
	}
	if err := ms.SetDenomPolicy(ctx, policy); err != nil {
		return nil, err
	}

	// Initialize cumulative lifetime tracking register
	if err := ms.SetMintedLifetime(ctx, denom, math.ZeroInt()); err != nil {
		return nil, err
	}

	// Record in CreatorIndex to guarantee creator-subdenom uniqueness
	if err := ms.SetCreatorSubdenom(ctx, creator.String(), msg.Subdenom); err != nil {
		return nil, err
	}

	// 4. Interact with the standard x/bank keeper interface to register the asset's official metadata (Name, Symbol, Decimals)
	name := msg.Name
	if strings.TrimSpace(name) == "" {
		name = msg.Symbol
	}
	symbol := msg.Symbol

	display := denom
	denomUnits := []*banktypes.DenomUnit{
		{
			Denom:    denom,
			Exponent: 0,
		},
	}

	if msg.Decimals > 0 {
		displayUnit := strings.ToLower(symbol)
		if sdk.ValidateDenom(displayUnit) == nil && displayUnit != denom {
			display = displayUnit
			denomUnits = append(denomUnits, &banktypes.DenomUnit{
				Denom:    display,
				Exponent: msg.Decimals,
			})
		}
	}

	metadata := banktypes.Metadata{
		Description: fmt.Sprintf("Factory token %s created by %s", denom, creator.String()),
		Base:        denom,
		Display:     display,
		Name:        name,
		Symbol:      symbol,
		DenomUnits:  denomUnits,
	}
	if err := metadata.Validate(); err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}
	ms.bankKeeper.SetDenomMetaData(ctx, metadata)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"create_denom",
		sdk.NewAttribute("denom", denom),
		sdk.NewAttribute("creator", creator.String()),
		sdk.NewAttribute("symbol", symbol),
		sdk.NewAttribute("supply_mode", supplyModeString(msg.SupplyMode)),
	))

	return &types.MsgCreateDenomResponse{Denom: denom}, nil
}

func supplyModeString(mode types.SupplyMode) string {
	switch mode {
	case types.SUPPLY_MODE_CAPPED:
		return "capped"
	case types.SUPPLY_MODE_UNCAPPED:
		return "uncapped"
	case types.SUPPLY_MODE_COLLATERALIZED:
		return "collateralized"
	default:
		return "unspecified"
	}
}
