package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/x/currencies/client/cli"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

func TestParseSupplyMode(t *testing.T) {
	testCases := []struct {
		input       string
		expected    types.SupplyMode
		expectError bool
	}{
		{"capped", types.SUPPLY_MODE_CAPPED, false},
		{"CAPPED", types.SUPPLY_MODE_CAPPED, false},
		{"1", types.SUPPLY_MODE_CAPPED, false},
		{"uncapped", types.SUPPLY_MODE_UNCAPPED, false},
		{"2", types.SUPPLY_MODE_UNCAPPED, false},
		{"collateralized", types.SUPPLY_MODE_COLLATERALIZED, false},
		{"3", types.SUPPLY_MODE_COLLATERALIZED, false},
		{"invalid", types.SUPPLY_MODE_UNSPECIFIED, true},
		{"", types.SUPPLY_MODE_UNSPECIFIED, true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			mode, err := cli.ParseSupplyMode(tc.input)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, mode)
			}
		})
	}
}

func TestCmdCreateDenomFlags(t *testing.T) {
	cmd := cli.CmdCreateDenom()
	require.NotNil(t, cmd)
	require.Equal(t, "create-denom [subdenom] [symbol]", cmd.Use)

	// Check custom flags exist with expected defaults
	supplyModeFlag := cmd.Flags().Lookup(cli.FlagSupplyMode)
	require.NotNil(t, supplyModeFlag)
	require.Equal(t, "capped", supplyModeFlag.DefValue)

	maxSupplyFlag := cmd.Flags().Lookup(cli.FlagMaxSupply)
	require.NotNil(t, maxSupplyFlag)
	require.Equal(t, "0", maxSupplyFlag.DefValue)

	feeBpsFlag := cmd.Flags().Lookup(cli.FlagTransferFeeBps)
	require.NotNil(t, feeBpsFlag)
	require.Equal(t, "0", feeBpsFlag.DefValue)

	canBurnFlag := cmd.Flags().Lookup(cli.FlagCanBurn)
	require.NotNil(t, canBurnFlag)
	require.Equal(t, "false", canBurnFlag.DefValue)

	collateralDenomFlag := cmd.Flags().Lookup(cli.FlagCollateralDenom)
	require.NotNil(t, collateralDenomFlag)
	require.Equal(t, "", collateralDenomFlag.DefValue)

	collateralRatioFlag := cmd.Flags().Lookup(cli.FlagCollateralRatio)
	require.NotNil(t, collateralRatioFlag)
	require.Equal(t, "1.0", collateralRatioFlag.DefValue)

	// Standard Tx flags added
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagFrom))
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagGas))
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagFees))
}

func TestCmdMintFlags(t *testing.T) {
	cmd := cli.CmdMint()
	require.NotNil(t, cmd)
	require.Equal(t, "mint [denom] [amount]", cmd.Use)

	recipientFlag := cmd.Flags().Lookup(cli.FlagRecipient)
	require.NotNil(t, recipientFlag)
	require.Equal(t, "", recipientFlag.DefValue)

	// Standard Tx flags added
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagFrom))
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagGas))
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagFees))
}

func TestCmdBurnFlags(t *testing.T) {
	cmd := cli.CmdBurn()
	require.NotNil(t, cmd)
	require.Equal(t, "burn [denom] [amount]", cmd.Use)

	// Standard Tx flags added
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagFrom))
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagGas))
	require.NotNil(t, cmd.Flags().Lookup(flags.FlagFees))
}

func TestGetTxCmd(t *testing.T) {
	rootCmd := cli.GetTxCmd()
	require.NotNil(t, rootCmd)
	require.Equal(t, types.ModuleName, rootCmd.Use)

	subcommands := rootCmd.Commands()
	require.Len(t, subcommands, 3)

	var subcmdNames []string
	for _, sub := range subcommands {
		subcmdNames = append(subcmdNames, sub.Name())
	}
	require.Contains(t, subcmdNames, "create-denom")
	require.Contains(t, subcmdNames, "mint")
	require.Contains(t, subcmdNames, "burn")
}
