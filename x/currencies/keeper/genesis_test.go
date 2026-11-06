package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

func TestInitAndExportGenesis(t *testing.T) {
	ctx, k, _, _ := setup(t)

	creator := sdk.AccAddress([]byte("creator_test_addr_12")).String()
	denom1 := types.FormatDenom(creator, "microtoken")
	denom2 := types.FormatDenom(creator, "stablecoin")

	policy1 := types.DenomPolicy{
		Denom:               denom1,
		Creator:             creator,
		Subdenom:            "microtoken",
		SupplyMode:          types.SupplyModeCapped,
		MaxSupply:           math.NewInt(1_000_000),
		CanBurn:             true,
		TransferFeeBps:      150,
		FeeCollectorAddress: creator,
	}

	policy2 := types.DenomPolicy{
		Denom:               denom2,
		Creator:             creator,
		Subdenom:            "stablecoin",
		SupplyMode:          types.SupplyModeCollateralized,
		CollateralDenom:     "unative",
		CollateralRatio:     math.LegacyMustNewDecFromStr("1.5"),
		TransferFeeBps:      50,
		FeeCollectorAddress: creator,
	}

	genesisState := types.NewGenesisState(
		[]types.DenomPolicy{policy1, policy2},
		[]types.CreatorIndexEntry{
			{Creator: creator, Subdenom: "microtoken"},
			{Creator: creator, Subdenom: "stablecoin"},
		},
		[]types.MintedLifetimeEntry{
			{Denom: denom1, Amount: math.NewInt(250_000)},
		},
	)

	// Verify stateless validation passes
	err := genesisState.Validate()
	require.NoError(t, err)

	// Run InitGenesis
	err = k.InitGenesis(ctx, genesisState)
	require.NoError(t, err)

	// Verify state in keeper
	storedPolicy1, err := k.GetDenomPolicy(ctx, denom1)
	require.NoError(t, err)
	require.Equal(t, policy1.Denom, storedPolicy1.Denom)
	require.Equal(t, policy1.MaxSupply, storedPolicy1.MaxSupply)

	storedPolicy2, err := k.GetDenomPolicy(ctx, denom2)
	require.NoError(t, err)
	require.Equal(t, policy2.CollateralDenom, storedPolicy2.CollateralDenom)

	has1, err := k.HasCreatorSubdenom(ctx, creator, "microtoken")
	require.NoError(t, err)
	require.True(t, has1)

	lifetime, err := k.GetMintedLifetime(ctx, denom1)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(250_000), lifetime)

	// Run ExportGenesis
	exported, err := k.ExportGenesis(ctx)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.Len(t, exported.DenomPolicies, 2)
	require.Len(t, exported.CreatorIndices, 2)
	require.Len(t, exported.MintedLifetimes, 1)
	require.Equal(t, math.NewInt(250_000), exported.MintedLifetimes[0].Amount)
}

func TestGenesisValidation(t *testing.T) {
	creator := sdk.AccAddress([]byte("creator_test_addr_12")).String()
	validDenom := types.FormatDenom(creator, "validtoken")

	// 1. Fee Bps exceeds limit
	invalidFeePolicy := types.DenomPolicy{
		Denom:          validDenom,
		Creator:        creator,
		Subdenom:       "validtoken",
		SupplyMode:     types.SupplyModeUncapped,
		TransferFeeBps: 1500, // > 1000 max
	}
	gsInvalidFee := types.NewGenesisState([]types.DenomPolicy{invalidFeePolicy}, nil, nil)
	require.Error(t, gsInvalidFee.Validate())

	// 2. Duplicate denoms
	dupPolicy1 := types.DenomPolicy{
		Denom:      validDenom,
		Creator:    creator,
		Subdenom:   "validtoken",
		SupplyMode: types.SupplyModeUncapped,
	}
	dupPolicy2 := types.DenomPolicy{
		Denom:      validDenom,
		Creator:    creator,
		Subdenom:   "validtoken",
		SupplyMode: types.SupplyModeUncapped,
	}
	gsDup := types.NewGenesisState([]types.DenomPolicy{dupPolicy1, dupPolicy2}, nil, nil)
	require.Error(t, gsDup.Validate())

	// 3. Malformed Bech32 fee collector
	badAddrPolicy := types.DenomPolicy{
		Denom:               validDenom,
		Creator:             creator,
		Subdenom:            "validtoken",
		SupplyMode:          types.SupplyModeUncapped,
		FeeCollectorAddress: "invalid_bech32",
	}
	gsBadAddr := types.NewGenesisState([]types.DenomPolicy{badAddrPolicy}, nil, nil)
	require.Error(t, gsBadAddr.Validate())

	// 4. Negative minted lifetime
	badMl := types.MintedLifetimeEntry{
		Denom:  validDenom,
		Amount: math.NewInt(-50),
	}
	gsBadMl := types.NewGenesisState(nil, nil, []types.MintedLifetimeEntry{badMl})
	require.Error(t, gsBadMl.Validate())
}
