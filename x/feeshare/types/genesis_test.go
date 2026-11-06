package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/feeshare/types"
)

func TestDefaultGenesisState(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NotNil(t, gs)

	// Validate default parameters
	require.Equal(t, uint64(2000), gs.Params.FounderBps)
	require.Equal(t, types.DefaultTreasuryAddress, gs.Params.TreasuryAddress)

	// Validate that default genesis passes validation
	err := gs.Validate()
	require.NoError(t, err)

	// Validate treasury address is a legitimate Bech32 account address
	addr, err := sdk.AccAddressFromBech32(gs.Params.TreasuryAddress)
	require.NoError(t, err)
	require.False(t, addr.Empty())
}

func TestGenesisStateValidate_Failures(t *testing.T) {
	testCases := []struct {
		name        string
		params      types.Params
		expectError bool
	}{
		{
			name:        "valid default params",
			params:      types.DefaultParams(),
			expectError: false,
		},
		{
			name: "valid custom params (1500 bps)",
			params: types.Params{
				FounderBps:      1500,
				TreasuryAddress: types.DefaultTreasuryAddress,
			},
			expectError: false,
		},
		{
			name: "valid max bps (10000 bps)",
			params: types.Params{
				FounderBps:      10000,
				TreasuryAddress: types.DefaultTreasuryAddress,
			},
			expectError: false,
		},
		{
			name: "invalid bps > 10000",
			params: types.Params{
				FounderBps:      10001,
				TreasuryAddress: types.DefaultTreasuryAddress,
			},
			expectError: true,
		},
		{
			name: "empty treasury address",
			params: types.Params{
				FounderBps:      2000,
				TreasuryAddress: "",
			},
			expectError: true,
		},
		{
			name: "invalid bech32 treasury",
			params: types.Params{
				FounderBps:      2000,
				TreasuryAddress: "not_a_bech32_address",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.NewGenesisState(tc.params)
			err := gs.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
