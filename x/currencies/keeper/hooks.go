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

type transferFeeContextKey struct{}

// SendRestrictionFn returns a banktypes.SendRestrictionFn that evaluates dynamic runtime hooks
// for factory tokens (pause checks, transfer fee deductions, and fee collector routing).
func (k Keeper) SendRestrictionFn() banktypes.SendRestrictionFn {
	return k.SendRestriction
}

// SendRestriction evaluates every transfer involving a factory/ namespaced denom.
func (k Keeper) SendRestriction(
	ctx context.Context,
	fromAddr, toAddr sdk.AccAddress,
	amt sdk.Coins,
) (sdk.AccAddress, error) {
	// Re-entrancy guard: if this transfer is an internal fee transfer, bypass hook
	if ctx.Value(transferFeeContextKey{}) != nil {
		return toAddr, nil
	}

	hasFactoryDenom := false
	for _, coin := range amt {
		if strings.HasPrefix(coin.Denom, types.DenomPrefix+"/") {
			hasFactoryDenom = true
			break
		}
	}
	if !hasFactoryDenom {
		return toAddr, nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for i := range amt {
		coin := &amt[i]
		if !strings.HasPrefix(coin.Denom, types.DenomPrefix+"/") {
			continue
		}

		// 1. Load the token's active DenomPolicy from storage
		policy, err := k.GetDenomPolicy(ctx, coin.Denom)
		if err != nil {
			return nil, errorsmod.Wrapf(err, "failed to load denom policy for %s", coin.Denom)
		}

		// 2. If policy.IsPaused evaluates to true, abort the operation immediately and throw an unauthorized error code
		if policy.IsPaused {
			return nil, errorsmod.Wrapf(
				sdkerrors.ErrUnauthorized,
				"token transfers are paused for denom %s",
				coin.Denom,
			)
		}

		// 4. Ensure that if transfer_fee_bps exceeds 1000 (10%), the runtime rejects the configuration to prevent complete balance seizures
		if policy.TransferFeeBps > types.MaxTransferFeeBps {
			return nil, errorsmod.Wrapf(
				sdkerrors.ErrInvalidRequest,
				"transfer_fee_bps %d exceeds maximum allowed limit %d (10%%) for denom %s",
				policy.TransferFeeBps, types.MaxTransferFeeBps, coin.Denom,
			)
		}

		// 3. If policy.TransferFeeBps is greater than zero, compute the fee using integer safety math: fee = amount * transfer_fee_bps / 10000
		if policy.TransferFeeBps > 0 && coin.Amount.IsPositive() {
			fee, remaining, err := ComputeTransferFee(coin.Amount, policy.TransferFeeBps)
			if err != nil {
				return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
			}

			if fee.IsPositive() {
				feeCollector := policy.FeeCollectorAddress
				if feeCollector == "" {
					feeCollector = policy.Admin
				}
				feeCollectorAddr, err := sdk.AccAddressFromBech32(feeCollector)
				if err != nil {
					return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid fee_collector_address %s: %s", feeCollector, err.Error())
				}

				// If fromAddr is already the fee collector or toAddr is fee collector, do not charge fee
				if !fromAddr.Equals(feeCollectorAddr) && !toAddr.Equals(feeCollectorAddr) {
					// 4. Subtract the fee from the principal amount
					coin.Amount = remaining

					// Transfer the fee fraction directly to the token's designated fee_collector_address
					feeCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, fee))
					internalCtx := context.WithValue(ctx, transferFeeContextKey{}, true)
					if err := k.bankKeeper.SendCoins(internalCtx, fromAddr, feeCollectorAddr, feeCoins); err != nil {
						return nil, errorsmod.Wrapf(err, "failed to transfer transfer fee of %s to fee collector %s", feeCoins, feeCollector)
					}

					sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
						"factory_transfer_fee",
						sdk.NewAttribute("denom", coin.Denom),
						sdk.NewAttribute("from", fromAddr.String()),
						sdk.NewAttribute("recipient", toAddr.String()),
						sdk.NewAttribute("fee_collector", feeCollectorAddr.String()),
						sdk.NewAttribute("fee", fee.String()),
						sdk.NewAttribute("remaining", remaining.String()),
						sdk.NewAttribute("transfer_fee_bps", fmt.Sprintf("%d", policy.TransferFeeBps)),
					))
				}
			}
		}
	}

	// Pass the remaining balance to the recipient account (SendCoins will transfer amt with modified coin.Amount)
	return toAddr, nil
}

// ComputeTransferFee computes the fee fraction and remaining balance using integer safety math:
// fee = amount * transfer_fee_bps / 10000
// remaining = amount - fee
func ComputeTransferFee(amount math.Int, transferFeeBps uint32) (fee math.Int, remaining math.Int, err error) {
	if transferFeeBps > types.MaxTransferFeeBps {
		return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("transfer_fee_bps %d exceeds maximum allowed limit %d (10%%)", transferFeeBps, types.MaxTransferFeeBps)
	}
	if transferFeeBps == 0 || !amount.IsPositive() {
		return math.ZeroInt(), amount, nil
	}

	bpsInt := math.NewIntFromUint64(uint64(transferFeeBps))
	tenThousand := math.NewInt(10000)

	feeProduct, err := amount.SafeMul(bpsInt)
	if err != nil {
		return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("overflow computing transfer fee product: %w", err)
	}

	fee = feeProduct.Quo(tenThousand)
	remaining, err = amount.SafeSub(fee)
	if err != nil {
		return math.ZeroInt(), math.ZeroInt(), fmt.Errorf("underflow computing remaining transfer balance: %w", err)
	}

	return fee, remaining, nil
}
