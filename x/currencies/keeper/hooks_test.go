package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/runtime"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/keeper"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

func setupHooksTest(t *testing.T) (sdk.Context, keeper.Keeper, *fakeBank) {
	t.Helper()
	key := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(key)
	ctx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test")).Ctx
	bank := newFakeBank()
	k := keeper.NewKeeper(storeService, bank)
	return ctx, k, bank
}

func TestSendRestriction_NonFactoryDenomPassThrough(t *testing.T) {
	ctx, k, _ := setupHooksTest(t)
	from := authtypes.NewModuleAddress("alice")
	to := authtypes.NewModuleAddress("bob")

	nativeCoins := sdk.NewCoins(sdk.NewCoin("unative", math.NewInt(5000)))
	newTo, err := k.SendRestriction(ctx, from, to, nativeCoins)
	require.NoError(t, err)
	require.Equal(t, to, newTo)
	require.Equal(t, math.NewInt(5000), nativeCoins[0].Amount)
}

func TestSendRestriction_PausedDenomAbortsWithUnauthorized(t *testing.T) {
	ctx, k, _ := setupHooksTest(t)
	admin := authtypes.NewModuleAddress("admin")
	from := authtypes.NewModuleAddress("alice")
	to := authtypes.NewModuleAddress("bob")

	denom := types.FormatDenom(admin.String(), "pausedtoken")
	err := k.SetDenomPolicy(ctx, types.DenomPolicy{
		Denom:               denom,
		Admin:               admin.String(),
		Symbol:              "PAUSE",
		SupplyMode:          types.SUPPLY_MODE_UNCAPPED,
		MaxSupply:           math.ZeroInt(),
		MintedLifetime:      math.ZeroInt(),
		IsPaused:            true,
		FeeCollectorAddress: admin.String(),
	})
	require.NoError(t, err)

	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(1000)))
	_, err = k.SendRestriction(ctx, from, to, coins)
	require.Error(t, err)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Contains(t, err.Error(), "token transfers are paused")
}

func TestSendRestriction_TransferFeeBpsExceeding1000Rejects(t *testing.T) {
	ctx, k, _ := setupHooksTest(t)
	admin := authtypes.NewModuleAddress("admin")
	from := authtypes.NewModuleAddress("alice")
	to := authtypes.NewModuleAddress("bob")

	denom := types.FormatDenom(admin.String(), "excessivefee")
	err := k.SetDenomPolicy(ctx, types.DenomPolicy{
		Denom:               denom,
		Admin:               admin.String(),
		Symbol:              "EXCESS",
		SupplyMode:          types.SUPPLY_MODE_UNCAPPED,
		MaxSupply:           math.ZeroInt(),
		MintedLifetime:      math.ZeroInt(),
		TransferFeeBps:      1001, // exceeds 1000 (10%)
		FeeCollectorAddress: admin.String(),
	})
	require.NoError(t, err)

	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(1000)))
	_, err = k.SendRestriction(ctx, from, to, coins)
	require.Error(t, err)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	require.Contains(t, err.Error(), "exceeds maximum allowed limit 1000")
}

func TestSendRestriction_TransferFeeDeductionAndRouting(t *testing.T) {
	ctx, k, bank := setupHooksTest(t)
	admin := authtypes.NewModuleAddress("admin")
	alice := authtypes.NewModuleAddress("alice")
	bob := authtypes.NewModuleAddress("bob")
	treasury := authtypes.NewModuleAddress("treasury")

	denom := types.FormatDenom(admin.String(), "usd")
	// 250 bps = 2.5% transfer fee
	err := k.SetDenomPolicy(ctx, types.DenomPolicy{
		Denom:               denom,
		Admin:               admin.String(),
		Symbol:              "USD",
		SupplyMode:          types.SUPPLY_MODE_UNCAPPED,
		MaxSupply:           math.ZeroInt(),
		MintedLifetime:      math.ZeroInt(),
		TransferFeeBps:      250,
		FeeCollectorAddress: treasury.String(),
	})
	require.NoError(t, err)

	// Alice has 10,000 tokens
	bank.SetBalance(alice, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(10000))))
	bank.SetBalance(treasury, sdk.NewCoins(sdk.NewCoin(denom, math.ZeroInt())))
	bank.SetBalance(bob, sdk.NewCoins(sdk.NewCoin(denom, math.ZeroInt())))

	transferCoins := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(10000)))

	// Execute SendRestriction hook
	newTo, err := k.SendRestriction(ctx, alice, bob, transferCoins)
	require.NoError(t, err)
	require.Equal(t, bob, newTo)

	// 1. Fee fraction = 10000 * 250 / 10000 = 250 tokens
	// Transferred directly to fee_collector_address (treasury)
	treasuryBal := bank.balances[treasury.String()].AmountOf(denom)
	require.Equal(t, math.NewInt(250), treasuryBal)

	// 2. Principal amount mutated in-place to remaining balance: 10000 - 250 = 9750
	require.Equal(t, math.NewInt(9750), transferCoins[0].Amount)

	// 3. Simulating standard SendCoins continuation:
	// Alice balance after fee transfer is 9,750; remaining 9,750 sent to Bob
	err = bank.SendCoins(ctx, alice, bob, transferCoins)
	require.NoError(t, err)

	aliceBal := bank.balances[alice.String()].AmountOf(denom)
	bobBal := bank.balances[bob.String()].AmountOf(denom)
	require.Equal(t, math.ZeroInt(), aliceBal)
	require.Equal(t, math.NewInt(9750), bobBal)
}

func TestSendRestriction_ZeroTransferFee(t *testing.T) {
	ctx, k, bank := setupHooksTest(t)
	admin := authtypes.NewModuleAddress("admin")
	alice := authtypes.NewModuleAddress("alice")
	bob := authtypes.NewModuleAddress("bob")

	denom := types.FormatDenom(admin.String(), "zerofee")
	err := k.SetDenomPolicy(ctx, types.DenomPolicy{
		Denom:               denom,
		Admin:               admin.String(),
		Symbol:              "ZERO",
		SupplyMode:          types.SUPPLY_MODE_UNCAPPED,
		MaxSupply:           math.ZeroInt(),
		MintedLifetime:      math.ZeroInt(),
		TransferFeeBps:      0,
		FeeCollectorAddress: admin.String(),
	})
	require.NoError(t, err)

	bank.SetBalance(alice, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(5000))))

	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(5000)))
	newTo, err := k.SendRestriction(ctx, alice, bob, coins)
	require.NoError(t, err)
	require.Equal(t, bob, newTo)
	require.Equal(t, math.NewInt(5000), coins[0].Amount)
}

func TestComputeTransferFee_UnitMath(t *testing.T) {
	// 1000 tokens with 250 bps (2.5%) -> fee = 25, remaining = 975
	fee, remaining, err := keeper.ComputeTransferFee(math.NewInt(1000), 250)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(25), fee)
	require.Equal(t, math.NewInt(975), remaining)

	// 1000 tokens with 1000 bps (10% ceiling) -> fee = 100, remaining = 900
	fee, remaining, err = keeper.ComputeTransferFee(math.NewInt(1000), 1000)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(100), fee)
	require.Equal(t, math.NewInt(900), remaining)

	// Small amount truncation (integer division): 3 tokens with 250 bps -> 3 * 250 / 10000 = 0 fee
	fee, remaining, err = keeper.ComputeTransferFee(math.NewInt(3), 250)
	require.NoError(t, err)
	require.Equal(t, math.ZeroInt(), fee)
	require.Equal(t, math.NewInt(3), remaining)

	// Reject fee exceeding 1000 bps (10%)
	_, _, err = keeper.ComputeTransferFee(math.NewInt(1000), 1001)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum allowed limit 1000")
}
