package types

import (
	"fmt"
	"regexp"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SupplyMode is the economic rule locked in at denom creation.
type SupplyMode int32

const (
	SUPPLY_MODE_UNSPECIFIED SupplyMode = iota
	SUPPLY_MODE_CAPPED
	SUPPLY_MODE_UNCAPPED
	SUPPLY_MODE_COLLATERALIZED

	SupplyModeUnspecified    = SUPPLY_MODE_UNSPECIFIED
	SupplyModeCapped         = SUPPLY_MODE_CAPPED
	SupplyModeUncapped       = SUPPLY_MODE_UNCAPPED
	SupplyModeCollateralized = SUPPLY_MODE_COLLATERALIZED
)

// DenomPolicy is the isolated state object for one factory denom.
// MintedLifetime is the cumulative amount ever minted. Burns do not reduce it,
// so a capped token cannot be refilled back up to MaxSupply.
type DenomPolicy struct {
	Denom               string         `json:"denom"`
	Creator             string         `json:"creator,omitempty"`
	Subdenom            string         `json:"subdenom,omitempty"`
	Admin               string         `json:"admin"`
	Symbol              string         `json:"symbol"`
	SupplyMode          SupplyMode     `json:"supply_mode"`
	MaxSupply           math.Int       `json:"max_supply"`
	MintedLifetime      math.Int       `json:"minted_lifetime"`
	TransferFeeBps      uint32         `json:"transfer_fee_bps,omitempty"`
	CanBurn             bool           `json:"can_burn,omitempty"`
	CollateralDenom     string         `json:"collateral_denom,omitempty"`
	CollateralRatio     math.LegacyDec `json:"collateral_ratio,omitempty"`
	IsPaused            bool           `json:"is_paused,omitempty"`
	FeeCollectorAddress string         `json:"fee_collector_address,omitempty"`
}

const (
	// MaxTransferFeeBps is the ceiling on per-transfer fees (1000 bps = 10%).
	// Transfers with fees exceeding this limit are rejected to prevent balance seizures.
	MaxTransferFeeBps uint32 = 1000
)

var (
	// subdenomRe keeps creator-chosen suffixes inside the bank denom charset.
	subdenomRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{2,44}$`)

	// AlphanumericSubdenomRe enforces that subdenom consists strictly of alphanumeric characters [a-zA-Z0-9].
	AlphanumericSubdenomRe = regexp.MustCompile(`^[a-zA-Z0-9]{1,44}$`)

	// DefaultCreateDenomStateRentGas charges a high fixed gas burn (2,000,000 gas units)
	// treating token deployment as long-term state rent.
	DefaultCreateDenomStateRentGas = uint64(2_000_000)
)

// ValidateAlphanumericSubdenom verifies that subdenom consists only of alphanumeric characters and is 1-44 chars.
func ValidateAlphanumericSubdenom(subdenom string) error {
	if len(subdenom) == 0 || len(subdenom) > 44 {
		return fmt.Errorf("subdenom length must be between 1 and 44 characters")
	}
	if !AlphanumericSubdenomRe.MatchString(subdenom) {
		return fmt.Errorf("subdenom %q must consist only of alphanumeric characters", subdenom)
	}
	return nil
}

// FormatDenom builds the bank denom owned by this module.
func FormatDenom(creator, subdenom string) string {
	return fmt.Sprintf("%s/%s/%s", DenomPrefix, creator, subdenom)
}

// ValidateSubdenom checks the creator-chosen suffix before it is embedded in a denom.
func ValidateSubdenom(subdenom string) error {
	if !subdenomRe.MatchString(subdenom) {
		return fmt.Errorf("subdenom %q must match %s", subdenom, subdenomRe.String())
	}
	return nil
}

// ValidateCreate checks the fields required to store a new policy.
func ValidateCreate(creator, subdenom, symbol string, mode SupplyMode, maxSupply math.Int) (string, error) {
	creatorAddr, err := sdk.AccAddressFromBech32(creator)
	if err != nil {
		return "", fmt.Errorf("invalid creator address: %w", err)
	}

	if err := ValidateSubdenom(subdenom); err != nil {
		return "", err
	}

	if symbol == "" || len(symbol) > 32 {
		return "", fmt.Errorf("symbol must be 1-32 characters")
	}

	switch mode {
	case SUPPLY_MODE_CAPPED:
		if maxSupply.IsNil() || !maxSupply.IsPositive() {
			return "", fmt.Errorf("capped denoms require a positive max_supply")
		}
	case SUPPLY_MODE_UNCAPPED, SUPPLY_MODE_COLLATERALIZED:
		if !maxSupply.IsNil() && !maxSupply.IsZero() {
			return "", fmt.Errorf("max_supply is only set for capped denoms")
		}
		maxSupply = math.ZeroInt()
	default:
		return "", fmt.Errorf("unsupported supply mode %d", mode)
	}

	denom := FormatDenom(creatorAddr.String(), subdenom)
	if err := sdk.ValidateDenom(denom); err != nil {
		return "", err
	}

	return denom, nil
}
