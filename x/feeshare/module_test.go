package feeshare_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/feeshare"
	"github.com/cosmos/cosmos-sdk/x/feeshare/types"
)

func setupCodec() codec.Codec {
	ir := codectypes.NewInterfaceRegistry()
	return codec.NewProtoCodec(ir)
}

func TestDefaultGenesis_HardcodedParameters(t *testing.T) {
	cdc := setupCodec()
	module := feeshare.NewAppModule(nil)

	bz := module.DefaultGenesis(cdc)
	require.NotEmpty(t, bz)

	var gs types.GenesisState
	err := json.Unmarshal(bz, &gs)
	require.NoError(t, err)

	// Invariant: founder_bps must initialize to 2000 (20%)
	require.Equal(t, uint64(2000), gs.Params.FounderBps)

	// Invariant: treasury_address must be a standard valid Bech32 address
	require.NotEmpty(t, gs.Params.TreasuryAddress)
	require.Equal(t, "aether1cxlt8kznps92fwu3j6npahx4mjfutydyay289x", gs.Params.TreasuryAddress)

	// Must be cryptographically valid Bech32
	addr, err := sdk.AccAddressFromBech32(gs.Params.TreasuryAddress)
	require.NoError(t, err)
	require.NotEmpty(t, addr)
}

func TestValidateGenesis_ValidState(t *testing.T) {
	cdc := setupCodec()
	module := feeshare.NewAppModule(nil)

	bz := module.DefaultGenesis(cdc)
	err := module.ValidateGenesis(cdc, nil, bz)
	require.NoError(t, err)
}

func TestValidateGenesis_InvalidParameters_PreventsBootup(t *testing.T) {
	cdc := setupCodec()
	module := feeshare.NewAppModule(nil)

	testCases := []struct {
		name        string
		params      types.Params
		expectError string
	}{
		{
			name: "founder_bps exceeds max (10001 bps)",
			params: types.Params{
				FounderBps:      10001,
				TreasuryAddress: types.DefaultTreasuryAddress,
			},
			expectError: "cannot exceed max basis points",
		},
		{
			name: "founder_bps exceeds max (20000 bps)",
			params: types.Params{
				FounderBps:      20000,
				TreasuryAddress: types.DefaultTreasuryAddress,
			},
			expectError: "cannot exceed max basis points",
		},
		{
			name: "empty treasury_address",
			params: types.Params{
				FounderBps:      2000,
				TreasuryAddress: "",
			},
			expectError: "treasury_address cannot be empty",
		},
		{
			name: "malformed treasury_address",
			params: types.Params{
				FounderBps:      2000,
				TreasuryAddress: "invalid_bech32_string",
			},
			expectError: "invalid treasury_address",
		},
		{
			name: "invalid bech32 checksum in treasury_address",
			params: types.Params{
				FounderBps:      2000,
				TreasuryAddress: "cosmos1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
			},
			expectError: "invalid treasury_address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.NewGenesisState(tc.params)
			bz, err := json.Marshal(gs)
			require.NoError(t, err)

			err = module.ValidateGenesis(cdc, nil, bz)
			require.Error(t, err, "expected ValidateGenesis to reject invalid parameters to prevent chain bootup")
			require.Contains(t, err.Error(), tc.expectError)
		})
	}

	t.Run("malformed json bytes", func(t *testing.T) {
		err := module.ValidateGenesis(cdc, nil, []byte("invalid-json"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal")
	})
}

func TestInitGenesis_PreventsBootupOnInvalidParams(t *testing.T) {
	cdc := setupCodec()
	module := feeshare.NewAppModule(nil)

	invalidGs := types.GenesisState{
		Params: types.Params{
			FounderBps:      15000, // Invalid: exceeds 10000
			TreasuryAddress: types.DefaultTreasuryAddress,
		},
	}
	bz, err := json.Marshal(invalidGs)
	require.NoError(t, err)

	ctx := sdk.Context{}
	require.Panics(t, func() {
		module.InitGenesis(ctx, cdc, bz)
	}, "expected InitGenesis to panic on invalid parameters to prevent chain bootup")
}
