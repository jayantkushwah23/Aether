package keeper_test

import (
	"context"
	"testing"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/runtime"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/keeper"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

type fakeBank struct {
	supply   map[string]math.Int
	metadata map[string]banktypes.Metadata
	minted   sdk.Coins
	sentTo   sdk.AccAddress
	sent     sdk.Coins
	burned   sdk.Coins
	balances map[string]sdk.Coins
}

func newFakeBank() *fakeBank {
	return &fakeBank{
		supply:   map[string]math.Int{},
		metadata: map[string]banktypes.Metadata{},
		balances: map[string]sdk.Coins{},
	}
}

func (f *fakeBank) MintCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	if moduleName != types.ModuleName {
		return sdkerrors.ErrUnknownRequest
	}
	f.minted = amt
	for _, coin := range amt {
		current := f.supply[coin.Denom]
		if current.IsNil() {
			current = math.ZeroInt()
		}
		f.supply[coin.Denom] = current.Add(coin.Amount)
	}
	return nil
}

func (f *fakeBank) BurnCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	if moduleName != types.ModuleName {
		return sdkerrors.ErrUnknownRequest
	}
	f.burned = amt
	for _, coin := range amt {
		current := f.supply[coin.Denom]
		if current.IsNil() {
			current = math.ZeroInt()
		}
		if current.LT(coin.Amount) {
			return errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "burn amount exceeds supply")
		}
		f.supply[coin.Denom] = current.Sub(coin.Amount)
	}
	return nil
}

func (f *fakeBank) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	if senderModule != types.ModuleName {
		return sdkerrors.ErrUnknownRequest
	}
	f.sentTo = recipientAddr
	f.sent = amt
	if f.balances != nil {
		userBal := f.balances[recipientAddr.String()]
		f.balances[recipientAddr.String()] = userBal.Add(amt...)
	}
	return nil
}

func (f *fakeBank) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	if recipientModule != types.ModuleName {
		return sdkerrors.ErrUnknownRequest
	}
	if f.balances != nil {
		userBal, ok := f.balances[senderAddr.String()]
		if !ok || !userBal.IsAllGTE(amt) {
			return errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "insufficient account balance")
		}
		f.balances[senderAddr.String()] = userBal.Sub(amt...)
	}
	return nil
}

func (f *fakeBank) SendCoins(_ context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error {
	if f.balances != nil {
		fromBal, ok := f.balances[fromAddr.String()]
		if !ok || !fromBal.IsAllGTE(amt) {
			return errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "insufficient funds for send")
		}
		f.balances[fromAddr.String()] = fromBal.Sub(amt...)
		toBal := f.balances[toAddr.String()]
		f.balances[toAddr.String()] = toBal.Add(amt...)
	}
	return nil
}

func (f *fakeBank) GetSupply(_ context.Context, denom string) sdk.Coin {
	amt := f.supply[denom]
	if amt.IsNil() {
		amt = math.ZeroInt()
	}
	return sdk.NewCoin(denom, amt)
}

func (f *fakeBank) SetDenomMetaData(_ context.Context, metadata banktypes.Metadata) {
	f.metadata[metadata.Base] = metadata
}

func (f *fakeBank) GetDenomMetaData(_ context.Context, denom string) (banktypes.Metadata, bool) {
	md, ok := f.metadata[denom]
	return md, ok
}

func (f *fakeBank) SetBalance(addr sdk.AccAddress, coins sdk.Coins) {
	f.balances[addr.String()] = coins
}

func setup(t *testing.T) (sdk.Context, keeper.Keeper, types.MsgServer, *fakeBank) {
	t.Helper()
	key := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(key)
	ctx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test")).Ctx
	bank := newFakeBank()
	k := keeper.NewKeeper(storeService, bank)
	return ctx, k, keeper.NewMsgServerImpl(k), bank
}

func TestCreateDenomStoresPolicy(t *testing.T) {
	ctx, k, srv, bank := setup(t)
	creator := authtypes.NewModuleAddress("alice")

	resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    creator.String(),
		Subdenom:   "usd",
		Symbol:     "USD",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(100),
	})
	require.NoError(t, err)

	wantDenom := types.FormatDenom(creator.String(), "usd")
	require.Equal(t, wantDenom, resp.Denom)

	policy, err := k.GetDenomPolicy(ctx, wantDenom)
	require.NoError(t, err)
	require.Equal(t, types.DenomPolicy{
		Denom:               wantDenom,
		Admin:               creator.String(),
		Symbol:              "USD",
		SupplyMode:          types.SUPPLY_MODE_CAPPED,
		MaxSupply:           math.NewInt(100),
		MintedLifetime:      math.ZeroInt(),
		CanBurn:             false,
		CollateralDenom:     "",
		CollateralRatio:     math.LegacyZeroDec(),
		IsPaused:            false,
		FeeCollectorAddress: creator.String(),
	}, policy)
	require.Equal(t, wantDenom, bank.metadata[wantDenom].Base)
}

func TestMintRejectsCappedSupplyAndLifetime(t *testing.T) {
	ctx, k, srv, bank := setup(t)
	admin := authtypes.NewModuleAddress("alice")
	denom := types.FormatDenom(admin.String(), "usd")

	_, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    admin.String(),
		Subdenom:   "usd",
		Symbol:     "USD",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(100),
	})
	require.NoError(t, err)

	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  admin.String(),
		Denom:  denom,
		Amount: math.NewInt(101),
	})
	require.ErrorIs(t, err, types.ErrSupplyCapExceeded)
	require.True(t, bank.minted.Empty())

	resp, err := srv.Mint(ctx, &types.MsgMint{
		Admin:  admin.String(),
		Denom:  denom,
		Amount: math.NewInt(60),
	})
	require.NoError(t, err)
	require.Equal(t, "60", resp.MintedLifetime)

	// A burn drops circulating supply without reducing minted_lifetime.
	bank.supply[denom] = math.NewInt(40)

	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  admin.String(),
		Denom:  denom,
		Amount: math.NewInt(50),
	})
	require.ErrorIs(t, err, types.ErrSupplyCapExceeded)

	policy, err := k.GetDenomPolicy(ctx, denom)
	require.NoError(t, err)
	require.True(t, policy.MintedLifetime.Equal(math.NewInt(60)))
	require.True(t, bank.supply[denom].Equal(math.NewInt(40)))
}

func TestMintAllowsExactCap(t *testing.T) {
	ctx, _, srv, bank := setup(t)
	admin := authtypes.NewModuleAddress("alice")
	recipient := authtypes.NewModuleAddress("bob")
	denom := types.FormatDenom(admin.String(), "usd")

	_, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    admin.String(),
		Subdenom:   "usd",
		Symbol:     "USD",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(100),
	})
	require.NoError(t, err)

	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:     admin.String(),
		Denom:     denom,
		Amount:    math.NewInt(100),
		Recipient: recipient.String(),
	})
	require.NoError(t, err)
	require.Equal(t, recipient, bank.sentTo)
	require.True(t, bank.supply[denom].Equal(math.NewInt(100)))
}

func TestCreateDenom_AlphanumericSubdenomValidation(t *testing.T) {
	ctx, _, srv, _ := setup(t)
	creator := authtypes.NewModuleAddress("alice")

	invalidSubdenoms := []string{
		"my-usd",
		"usd!",
		"usd/v1",
		"usd space",
		"@usd",
		"",
	}

	for _, sub := range invalidSubdenoms {
		t.Run("invalid_"+sub, func(t *testing.T) {
			_, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
				Creator:    creator.String(),
				Subdenom:   sub,
				Symbol:     "USD",
				SupplyMode: types.SUPPLY_MODE_CAPPED,
				MaxSupply:  math.NewInt(1000),
			})
			require.Error(t, err)
			require.ErrorIs(t, err, types.ErrInvalidSubdenom)
		})
	}

	validSubdenoms := []string{
		"usd",
		"USD123",
		"token99",
		"gold",
	}

	for _, sub := range validSubdenoms {
		t.Run("valid_"+sub, func(t *testing.T) {
			resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
				Creator:    creator.String(),
				Subdenom:   sub,
				Symbol:     "TKN",
				SupplyMode: types.SUPPLY_MODE_CAPPED,
				MaxSupply:  math.NewInt(1000),
			})
			require.NoError(t, err)
			require.Equal(t, types.FormatDenom(creator.String(), sub), resp.Denom)
		})
	}
}

func TestCreateDenom_CreatorIndexDuplicateRejection(t *testing.T) {
	ctx, k, srv, _ := setup(t)
	alice := authtypes.NewModuleAddress("alice")
	bob := authtypes.NewModuleAddress("bob")

	// 1. Alice creates "usd"
	_, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    alice.String(),
		Subdenom:   "usd",
		Symbol:     "USD",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(1000),
	})
	require.NoError(t, err)

	// Verify CreatorIndex is populated for Alice
	has, err := k.HasCreatorSubdenom(ctx, alice.String(), "usd")
	require.NoError(t, err)
	require.True(t, has)

	// 2. Alice tries to create "usd" again -> rejected by CreatorIndex
	_, err = srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    alice.String(),
		Subdenom:   "usd",
		Symbol:     "USD2",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(500),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDenomExists)

	// 3. Bob CAN create "usd" in his own isolated namespace
	respBob, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    bob.String(),
		Subdenom:   "usd",
		Symbol:     "USD",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(2000),
	})
	require.NoError(t, err)
	require.Equal(t, types.FormatDenom(bob.String(), "usd"), respBob.Denom)

	hasBob, err := k.HasCreatorSubdenom(ctx, bob.String(), "usd")
	require.NoError(t, err)
	require.True(t, hasBob)
}

func TestCreateDenom_BankMetadataWithDecimalsAndStateRentGas(t *testing.T) {
	ctx, _, srv, bank := setup(t)
	creator := authtypes.NewModuleAddress("alice")

	initialGas := ctx.GasMeter().GasConsumed()

	resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    creator.String(),
		Subdenom:   "stable",
		Symbol:     "USD",
		Name:       "United States Dollar",
		Decimals:   6,
		SupplyMode: types.SUPPLY_MODE_UNCAPPED,
	})
	require.NoError(t, err)

	denom := resp.Denom
	metadata, ok := bank.metadata[denom]
	require.True(t, ok)
	require.Equal(t, "United States Dollar", metadata.Name)
	require.Equal(t, "USD", metadata.Symbol)
	require.Equal(t, denom, metadata.Base)
	require.Equal(t, "usd", metadata.Display)
	require.Len(t, metadata.DenomUnits, 2)
	require.Equal(t, uint32(0), metadata.DenomUnits[0].Exponent)
	require.Equal(t, uint32(6), metadata.DenomUnits[1].Exponent)

	// Invariant: Verify high fixed state rent gas consumed
	gasConsumed := ctx.GasMeter().GasConsumed() - initialGas
	require.GreaterOrEqual(t, gasConsumed, types.DefaultCreateDenomStateRentGas)
}

func TestCappedSupply_CanBurnTrue_BurnRestoresHeadroom(t *testing.T) {
	ctx, k, srv, bank := setup(t)
	alice := authtypes.NewModuleAddress("alice")

	resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    alice.String(),
		Subdenom:   "cappedburn",
		Symbol:     "CAPB",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(100),
		CanBurn:    true,
	})
	require.NoError(t, err)
	denom := resp.Denom

	// 1. Mint up to exact cap
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(100),
	})
	require.NoError(t, err)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(100)))

	// 2. Reject mint beyond cap
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(1),
	})
	require.ErrorIs(t, err, types.ErrSupplyCapExceeded)

	// 3. Burn 30 tokens from Alice
	bank.SetBalance(alice, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(100))))
	burnResp, err := srv.Burn(ctx, &types.MsgBurn{
		Sender: alice.String(),
		Denom:  denom,
		Amount: math.NewInt(30),
	})
	require.NoError(t, err)
	require.Equal(t, "30", burnResp.Burned)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(70)))

	// 4. Because CanBurn is true, burning restored 30 units of minting headroom
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(30),
	})
	require.NoError(t, err)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(100)))

	// 5. Minting 1 more fails again
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(1),
	})
	require.ErrorIs(t, err, types.ErrSupplyCapExceeded)

	_ = k
}

func TestCappedSupply_CanBurnFalse_MintedLifetimeRegisterBlocksHeadroom(t *testing.T) {
	ctx, k, srv, bank := setup(t)
	alice := authtypes.NewModuleAddress("alice")

	resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    alice.String(),
		Subdenom:   "cappednoburn",
		Symbol:     "CAPNB",
		SupplyMode: types.SUPPLY_MODE_CAPPED,
		MaxSupply:  math.NewInt(100),
		CanBurn:    false,
	})
	require.NoError(t, err)
	denom := resp.Denom

	// 1. Mint 60 tokens
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(60),
	})
	require.NoError(t, err)

	lifetime, err := k.GetMintedLifetime(ctx, denom)
	require.NoError(t, err)
	require.True(t, lifetime.Equal(math.NewInt(60)))

	// 2. Burning when CanBurn=false is rejected
	bank.SetBalance(alice, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(60))))
	_, err = srv.Burn(ctx, &types.MsgBurn{
		Sender: alice.String(),
		Denom:  denom,
		Amount: math.NewInt(20),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrBurnDisabled)

	// 3. Even if external bank burn dropped supply to 20, lifetime register enforces the cap
	bank.supply[denom] = math.NewInt(20)

	// Trying to mint 50 fails because lifetime(60) + 50 = 110 > 100
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(50),
	})
	require.ErrorIs(t, err, types.ErrSupplyCapExceeded)

	// 4. Minting 40 succeeds (lifetime reaches exactly 100)
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(40),
	})
	require.NoError(t, err)

	lifetime, err = k.GetMintedLifetime(ctx, denom)
	require.NoError(t, err)
	require.True(t, lifetime.Equal(math.NewInt(100)))

	// 5. Subsequent mint of 1 is rejected
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(1),
	})
	require.ErrorIs(t, err, types.ErrSupplyCapExceeded)
}

func TestUncappedSupply_AdminRestrictedMintAndBurn(t *testing.T) {
	ctx, _, srv, bank := setup(t)
	alice := authtypes.NewModuleAddress("alice")
	bob := authtypes.NewModuleAddress("bob")

	resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:    alice.String(),
		Subdenom:   "gold",
		Symbol:     "GLD",
		SupplyMode: types.SUPPLY_MODE_UNCAPPED,
	})
	require.NoError(t, err)
	denom := resp.Denom

	// 1. Non-admin Bob cannot mint
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  bob.String(),
		Denom:  denom,
		Amount: math.NewInt(1000),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)

	// 2. Admin Alice mints 500,000 to herself
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  alice.String(),
		Denom:  denom,
		Amount: math.NewInt(500000),
	})
	require.NoError(t, err)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(500000)))

	// 3. Admin Alice mints 200,000 to Bob
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:     alice.String(),
		Denom:     denom,
		Amount:    math.NewInt(200000),
		Recipient: bob.String(),
	})
	require.NoError(t, err)
	require.Equal(t, bob, bank.sentTo)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(700000)))

	// 4. Non-admin Bob cannot burn
	_, err = srv.Burn(ctx, &types.MsgBurn{
		Sender: bob.String(),
		Denom:  denom,
		Amount: math.NewInt(100),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)

	// 5. Admin Alice burns 100,000 directly from account balance
	bank.SetBalance(alice, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(500000))))
	burnResp, err := srv.Burn(ctx, &types.MsgBurn{
		Sender: alice.String(),
		Denom:  denom,
		Amount: math.NewInt(100000),
	})
	require.NoError(t, err)
	require.Equal(t, "100000", burnResp.Burned)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(600000)))
}

func TestCollateralizedSupply_EscrowTransferAndMintBurn(t *testing.T) {
	ctx, _, srv, bank := setup(t)
	alice := authtypes.NewModuleAddress("alice")
	bob := authtypes.NewModuleAddress("bob")

	resp, err := srv.CreateDenom(ctx, &types.MsgCreateDenom{
		Creator:         alice.String(),
		Subdenom:        "stable",
		Symbol:          "STBL",
		SupplyMode:      types.SUPPLY_MODE_COLLATERALIZED,
		CollateralDenom: "unative",
		CollateralRatio: math.LegacyMustNewDecFromStr("1.5"),
	})
	require.NoError(t, err)
	denom := resp.Denom

	// Fund Bob with 300 unative
	bank.SetBalance(bob, sdk.NewCoins(sdk.NewCoin("unative", math.NewInt(300))))

	// 1. Bob mints 100 STBL tokens.
	// Required collateral = ceil(100 * 1.5) = 150 unative.
	mintResp, err := srv.Mint(ctx, &types.MsgMint{
		Admin:  bob.String(),
		Denom:  denom,
		Amount: math.NewInt(100),
	})
	require.NoError(t, err)
	require.Equal(t, denom, mintResp.Denom)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(100)))

	// Check Bob's remaining unative: 300 - 150 = 150
	bobUnative := bank.balances[bob.String()].AmountOf("unative")
	require.True(t, bobUnative.Equal(math.NewInt(150)))

	// 2. Bob attempts to mint 200 STBL (requires 300 unative, Bob only has 150)
	_, err = srv.Mint(ctx, &types.MsgMint{
		Admin:  bob.String(),
		Denom:  denom,
		Amount: math.NewInt(200),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientCollateral)

	// 3. Bob burns 50 STBL tokens.
	// Required escrow release = floor(50 * 1.5) = 75 unative.
	// Give Bob the 50 STBL in balance to burn
	bank.SetBalance(bob, sdk.NewCoins(
		sdk.NewCoin("unative", math.NewInt(150)),
		sdk.NewCoin(denom, math.NewInt(100)),
	))

	burnResp, err := srv.Burn(ctx, &types.MsgBurn{
		Sender: bob.String(),
		Denom:  denom,
		Amount: math.NewInt(50),
	})
	require.NoError(t, err)
	require.Equal(t, "50", burnResp.Burned)
	require.True(t, bank.GetSupply(ctx, denom).Amount.Equal(math.NewInt(50)))

	// Escrowed unative returned to Bob: 150 + 75 = 225
	bobUnativeAfterBurn := bank.balances[bob.String()].AmountOf("unative")
	require.True(t, bobUnativeAfterBurn.Equal(math.NewInt(225)))
}
