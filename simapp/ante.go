package simapp

import (
	"bytes"
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	feesharetypes "github.com/cosmos/cosmos-sdk/x/feeshare/types"
)

// FeeshareKeeper defines the interface required to query feeshare parameters dynamically.
type FeeshareKeeper interface {
	GetParams(ctx context.Context) (feesharetypes.Params, error)
}

// FounderFeeSplitDecorator intercepts transaction fee deduction and splits the native
// gas fees dynamically: querying the feeshare keeper to route founder_bps (e.g. 20%)
// directly to the designated on-chain founder treasury address, leaving the remaining
// balance to follow the normal x/distribution validator pool pathway.
type FounderFeeSplitDecorator struct {
	accountKeeper      ante.AccountKeeper
	bankKeeper         authtypes.BankKeeper
	feegrantKeeper     ante.FeegrantKeeper
	feeshareKeeper     FeeshareKeeper
	txFeeChecker       ante.TxFeeChecker
	feeRecipientModule string
}

// NewFounderFeeSplitDecorator constructs the dynamic fee splitter decorator.
func NewFounderFeeSplitDecorator(
	ak ante.AccountKeeper,
	bk authtypes.BankKeeper,
	fk ante.FeegrantKeeper,
	fsk FeeshareKeeper,
	tfc ante.TxFeeChecker,
) FounderFeeSplitDecorator {
	if tfc == nil {
		tfc = checkTxFeeWithValidatorMinGasPrices
	}

	return FounderFeeSplitDecorator{
		accountKeeper:      ak,
		bankKeeper:         bk,
		feegrantKeeper:     fk,
		feeshareKeeper:     fsk,
		txFeeChecker:       tfc,
		feeRecipientModule: authtypes.FeeCollectorName,
	}
}

// AnteHandle executes the fee deduction and split.
func (s FounderFeeSplitDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	if !simulate && ctx.BlockHeight() > 0 && feeTx.GetGas() == 0 {
		return ctx, errorsmod.Wrap(sdkerrors.ErrInvalidGasLimit, "must provide positive gas")
	}

	var (
		priority int64
		err      error
	)

	fee := feeTx.GetFee()
	if !simulate {
		fee, priority, err = s.txFeeChecker(ctx, tx)
		if err != nil {
			return ctx, err
		}
	}
	if err := s.checkDeductFee(ctx, tx, fee); err != nil {
		return ctx, err
	}

	newCtx := ctx.WithPriority(priority)
	return next(newCtx, tx, simulate)
}

func (s FounderFeeSplitDecorator) checkDeductFee(ctx sdk.Context, sdkTx sdk.Tx, fee sdk.Coins) error {
	feeTx, ok := sdkTx.(sdk.FeeTx)
	if !ok {
		return errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	if addr := s.accountKeeper.GetModuleAddress(s.feeRecipientModule); addr == nil {
		return fmt.Errorf("fee recipient module account (%s) has not been set", s.feeRecipientModule)
	}

	feePayer := feeTx.FeePayer()
	feeGranter := feeTx.FeeGranter()
	deductFeesFrom := feePayer

	// If fee granter set, deduct fee from feegranter account
	if feeGranter != nil {
		feeGranterAddr := sdk.AccAddress(feeGranter)
		if s.feegrantKeeper == nil {
			return sdkerrors.ErrInvalidRequest.Wrap("fee grants are not enabled")
		} else if !bytes.Equal(feeGranterAddr, feePayer) {
			err := s.feegrantKeeper.UseGrantedFees(ctx, feeGranterAddr, feePayer, fee, sdkTx.GetMsgs())
			if err != nil {
				return errorsmod.Wrapf(err, "%s does not allow to pay fees for %s", feeGranter, feePayer)
			}
		}
		deductFeesFrom = feeGranterAddr
	}

	deductFeesFromAcc := s.accountKeeper.GetAccount(ctx, deductFeesFrom)
	if deductFeesFromAcc == nil {
		return sdkerrors.ErrUnknownAddress.Wrapf("fee payer address: %s does not exist", deductFeesFrom)
	}

	// Deduct and split fees if non-zero
	if !fee.IsZero() {
		if !fee.IsValid() {
			return errorsmod.Wrapf(sdkerrors.ErrInsufficientFee, "invalid fee amount: %s", fee)
		}

		// Dynamically query feeshare keeper for parameters
		var (
			founderBps      uint64
			treasuryAddress sdk.AccAddress
		)
		if s.feeshareKeeper != nil {
			params, err := s.feeshareKeeper.GetParams(ctx)
			if err != nil {
				params = feesharetypes.DefaultParams()
			}
			founderBps = params.FounderBps
			if params.TreasuryAddress != "" {
				addr, addrErr := sdk.AccAddressFromBech32(params.TreasuryAddress)
				if addrErr == nil {
					treasuryAddress = addr
				}
			}
		}

		var (
			founderCoins   sdk.Coins
			validatorCoins sdk.Coins
		)

		tenThousand := math.NewInt(10000)
		founderBpsInt := math.NewIntFromUint64(founderBps)

		for _, coin := range fee {
			var founderAmt math.Int
			if founderBps > 0 && len(treasuryAddress) > 0 {
				founderAmt = coin.Amount.Mul(founderBpsInt).Quo(tenThousand)
			} else {
				founderAmt = math.ZeroInt()
			}
			validatorAmt := coin.Amount.Sub(founderAmt)

			if founderAmt.IsPositive() {
				founderCoins = founderCoins.Add(sdk.NewCoin(coin.Denom, founderAmt))
			}
			if validatorAmt.IsPositive() {
				validatorCoins = validatorCoins.Add(sdk.NewCoin(coin.Denom, validatorAmt))
			}
		}

		// 1. Send founder fraction directly to the dynamically configured founder treasury address
		if !founderCoins.IsZero() && len(treasuryAddress) > 0 {
			if err := s.bankKeeper.SendCoins(ctx, deductFeesFromAcc.GetAddress(), treasuryAddress, founderCoins); err != nil {
				return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, "failed to route founder fee %s to treasury %s: %s", founderCoins, treasuryAddress.String(), err.Error())
			}
		}

		// 2. Send remaining balance to standard fee_collector module account for validator pool distribution
		if !validatorCoins.IsZero() {
			if err := s.bankKeeper.SendCoinsFromAccountToModule(ctx, deductFeesFromAcc.GetAddress(), s.feeRecipientModule, validatorCoins); err != nil {
				return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, "failed to route validator fee %s: %s", validatorCoins, err.Error())
			}
		}
	}

	events := sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeTx,
			sdk.NewAttribute(sdk.AttributeKeyFee, fee.String()),
			sdk.NewAttribute(sdk.AttributeKeyFeePayer, sdk.AccAddress(deductFeesFrom).String()),
		),
	}
	ctx.EventManager().EmitEvents(events)

	return nil
}

// checkTxFeeWithValidatorMinGasPrices implements fallback fee checking based on validator min gas prices.
func checkTxFeeWithValidatorMinGasPrices(ctx sdk.Context, tx sdk.Tx) (sdk.Coins, int64, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return nil, 0, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	feeCoins := feeTx.GetFee()
	gas := feeTx.GetGas()

	if ctx.IsCheckTx() {
		minGasPrices := ctx.MinGasPrices()
		if !minGasPrices.IsZero() {
			requiredFees := make(sdk.Coins, len(minGasPrices))
			glDec := math.LegacyNewDec(int64(gas))

			for i, gp := range minGasPrices {
				fee := gp.Amount.Mul(glDec)
				requiredFees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
			}

			if !feeCoins.IsAnyGTE(requiredFees) {
				return nil, 0, errorsmod.Wrapf(sdkerrors.ErrInsufficientFee, "insufficient fees; got: %s required: %s", feeCoins, requiredFees)
			}
		}
	}

	priority := getTxPriority(feeCoins, int64(gas))
	return feeCoins, priority, nil
}

func getTxPriority(fee sdk.Coins, gas int64) int64 {
	var priority int64
	for _, c := range fee {
		p := c.Amount.Int64()
		if gas > 0 {
			p = p / gas
		}
		if p > priority {
			priority = p
		}
	}
	return priority
}

// NewAnteHandler returns the AnteHandler chain with the dynamic FounderFeeSplitDecorator
// directly replacing the standard fee decorator.
func NewAnteHandler(options ante.HandlerOptions, feeshareKeeper FeeshareKeeper) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "account keeper is required for ante builder")
	}
	if options.BankKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "bank keeper is required for ante builder")
	}
	if options.SignModeHandler == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "sign mode handler is required for ante builder")
	}

	anteDecorators := []sdk.AnteDecorator{
		ante.NewSetUpContextDecorator(),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		NewFounderFeeSplitDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, feeshareKeeper, options.TxFeeChecker),
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler, options.SigVerifyOptions...),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}
