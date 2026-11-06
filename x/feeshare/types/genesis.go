package types

import "fmt"

// GenesisState defines the feeshare module's genesis state.
type GenesisState struct {
	Params Params `json:"params"`
}

// NewGenesisState creates a new GenesisState object.
func NewGenesisState(params Params) *GenesisState {
	return &GenesisState{
		Params: params,
	}
}

// DefaultGenesis returns the default genesis state initialized with hardcoded defaults:
// - founder_bps: 2000 (20%)
// - treasury_address: standard valid Bech32 address
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs stateless validation of the genesis state.
// Returns an error if any parameter invariant is violated.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid feeshare genesis state: %w", err)
	}
	return nil
}
