package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DefaultFounderBps is 2000 basis points (20.00%)
	DefaultFounderBps uint64 = 2000

	// DefaultTreasuryAddress is a standard valid Bech32 address used as the default founder treasury.
	DefaultTreasuryAddress string = "aether1cxlt8kznps92fwu3j6npahx4mjfutydyay289x"

	// MaxFounderBps represents 100% in basis points (10000 bps)
	MaxFounderBps uint64 = 10000
)

// Params defines the configuration parameters for the feeshare module.
type Params struct {
	FounderBps      uint64 `json:"founder_bps"`
	TreasuryAddress string `json:"treasury_address"`
}

// NewParams creates a new Params instance.
func NewParams(founderBps uint64, treasuryAddress string) Params {
	return Params{
		FounderBps:      founderBps,
		TreasuryAddress: treasuryAddress,
	}
}

// DefaultParams returns the default parameters with founder_bps initialized to 2000 (20%)
// and treasury_address set to a standard valid Bech32 address.
func DefaultParams() Params {
	return Params{
		FounderBps:      DefaultFounderBps,
		TreasuryAddress: DefaultTreasuryAddress,
	}
}

// Validate checks that the feeshare parameters meet all consensus requirements.
// If founder_bps exceeds 10000 or treasury_address is not a valid Bech32 address, an error is returned.
func (p Params) Validate() error {
	if p.FounderBps > MaxFounderBps {
		return fmt.Errorf("founder_bps (%d) cannot exceed max basis points (%d)", p.FounderBps, MaxFounderBps)
	}

	if p.TreasuryAddress == "" {
		return fmt.Errorf("treasury_address cannot be empty")
	}

	if _, err := sdk.AccAddressFromBech32(p.TreasuryAddress); err != nil {
		return fmt.Errorf("invalid treasury_address %q: %w", p.TreasuryAddress, err)
	}

	return nil
}
