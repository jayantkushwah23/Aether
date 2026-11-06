package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

// Flag definitions for currencies tx commands
const (
	FlagSupplyMode      = "supply-mode"
	FlagMaxSupply       = "max-supply"
	FlagTransferFeeBps  = "transfer-fee-bps"
	FlagRecipient       = "recipient"
	FlagCanBurn         = "can-burn"
	FlagCollateralDenom = "collateral-denom"
	FlagCollateralRatio = "collateral-ratio"
)

// GetTxCmd returns the root transaction command for the currencies module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Currencies transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdCreateDenom(),
		CmdMint(),
		CmdBurn(),
	)

	return cmd
}

// CmdCreateDenom returns a CLI command to deploy and register a new factory denom.
func CmdCreateDenom() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-denom [subdenom] [symbol]",
		Short: "Deploy an independent token with dynamic supply rules and custom economic policy",
		Long: strings.TrimSpace(`Deploy a new independent token denom dynamically under the creator's isolated namespace:
  factory/{creator_address}/{subdenom}

Flags allow configuring the supply mode (capped, uncapped, collateralized), 
the cumulative lifetime maximum supply, and per-transfer transaction fees.

Examples:
  $ simapp tx currencies create-denom myusd USD --supply-mode capped --max-supply 1000000000 --from mykey
  $ simapp tx currencies create-denom gold GOLD --supply-mode uncapped --transfer-fee-bps 250 --from mykey
  $ simapp tx currencies create-denom usdc USDC --supply-mode collateralized --from mykey
`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			subdenom := args[0]
			symbol := args[1]

			supplyModeStr, err := cmd.Flags().GetString(FlagSupplyMode)
			if err != nil {
				return err
			}

			supplyMode, err := ParseSupplyMode(supplyModeStr)
			if err != nil {
				return err
			}

			maxSupplyStr, err := cmd.Flags().GetString(FlagMaxSupply)
			if err != nil {
				return err
			}

			var maxSupply math.Int
			if maxSupplyStr != "" && maxSupplyStr != "0" {
				var ok bool
				maxSupply, ok = math.NewIntFromString(maxSupplyStr)
				if !ok {
					return fmt.Errorf("invalid max-supply value %q: must be a positive integer", maxSupplyStr)
				}
			} else {
				maxSupply = math.ZeroInt()
			}

			transferFeeBps, err := cmd.Flags().GetUint32(FlagTransferFeeBps)
			if err != nil {
				return err
			}

			canBurn, err := cmd.Flags().GetBool(FlagCanBurn)
			if err != nil {
				return err
			}

			collateralDenom, err := cmd.Flags().GetString(FlagCollateralDenom)
			if err != nil {
				return err
			}

			collateralRatioStr, err := cmd.Flags().GetString(FlagCollateralRatio)
			if err != nil {
				return err
			}

			var collateralRatio math.LegacyDec
			if collateralRatioStr != "" && collateralRatioStr != "0" {
				collateralRatio, err = math.LegacyNewDecFromStr(collateralRatioStr)
				if err != nil {
					return fmt.Errorf("invalid collateral-ratio %q: %w", collateralRatioStr, err)
				}
			} else {
				collateralRatio = math.LegacyZeroDec()
			}

			creator := clientCtx.GetFromAddress().String()

			msg := &types.MsgCreateDenom{
				Creator:         creator,
				Subdenom:        subdenom,
				Symbol:          symbol,
				SupplyMode:      supplyMode,
				MaxSupply:       maxSupply,
				TransferFeeBps:  transferFeeBps,
				CanBurn:         canBurn,
				CollateralDenom: collateralDenom,
				CollateralRatio: collateralRatio,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String(FlagSupplyMode, "capped", "Supply mode: 'capped', 'uncapped', or 'collateralized'")
	cmd.Flags().String(FlagMaxSupply, "0", "Cumulative maximum supply ceiling (required for capped supply mode)")
	cmd.Flags().Uint32(FlagTransferFeeBps, 0, "Per-transfer transaction fee in basis points (e.g. 250 = 2.5%)")
	cmd.Flags().Bool(FlagCanBurn, false, "Enable burn capabilities for capped denoms (allows burns to restore minting headroom)")
	cmd.Flags().String(FlagCollateralDenom, "", "Denom used as escrow collateral for collateralized supply mode")
	cmd.Flags().String(FlagCollateralRatio, "1.0", "Collateral ratio required to mint (e.g. 1.5 = 150% collateral)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// CmdMint returns a CLI command to mint new units of an existing factory denom.
func CmdMint() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mint [denom] [amount]",
		Short: "Mint new units of a factory denom to an account",
		Long: strings.TrimSpace(`Mint units of an existing factory denom.
The transaction signer (--from) must match the registered administrator of the denom policy.
If --recipient is not set, the newly minted coins will be transferred directly to the admin.

Examples:
  $ simapp tx currencies mint factory/cosmos1.../myusd 500000 --from mykey
  $ simapp tx currencies mint factory/cosmos1.../myusd 500000 --recipient cosmos1... --from mykey
`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]
			amountStr := args[1]

			amount, ok := math.NewIntFromString(amountStr)
			if !ok || !amount.IsPositive() {
				return fmt.Errorf("invalid mint amount %q: must be a positive integer", amountStr)
			}

			recipient, err := cmd.Flags().GetString(FlagRecipient)
			if err != nil {
				return err
			}

			admin := clientCtx.GetFromAddress().String()
			if recipient == "" {
				recipient = admin
			}

			msg := &types.MsgMint{
				Admin:     admin,
				Denom:     denom,
				Amount:    amount,
				Recipient: recipient,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String(FlagRecipient, "", "Optional recipient address (defaults to admin address if omitted)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// CmdBurn returns a CLI command to burn units of an existing factory denom from the user's account balance.
func CmdBurn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn [denom] [amount]",
		Short: "Burn units of a factory denom from your account balance",
		Long: strings.TrimSpace(`Burn units of an existing factory denom from the signer's account balance.
In capped mode (with can_burn enabled), burning reduces supply and restores minting headroom.
In uncapped mode, only the admin may execute burns.
In collateralized mode, burning releases the proportional escrowed collateral assets.

Examples:
  $ simapp tx currencies burn factory/cosmos1.../myusd 500000 --from mykey
`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]
			amountStr := args[1]

			amount, ok := math.NewIntFromString(amountStr)
			if !ok || !amount.IsPositive() {
				return fmt.Errorf("invalid burn amount %q: must be a positive integer", amountStr)
			}

			sender := clientCtx.GetFromAddress().String()

			msg := &types.MsgBurn{
				Sender: sender,
				Denom:  denom,
				Amount: amount,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// ParseSupplyMode parses user-supplied string representation into a types.SupplyMode.
func ParseSupplyMode(mode string) (types.SupplyMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "capped", "1":
		return types.SUPPLY_MODE_CAPPED, nil
	case "uncapped", "2":
		return types.SUPPLY_MODE_UNCAPPED, nil
	case "collateralized", "3":
		return types.SUPPLY_MODE_COLLATERALIZED, nil
	default:
		return types.SUPPLY_MODE_UNSPECIFIED, fmt.Errorf("unrecognized supply mode %q (must be 'capped', 'uncapped', or 'collateralized')", mode)
	}
}
