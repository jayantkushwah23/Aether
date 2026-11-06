package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CreatorIndexEntry represents a registered creator and subdenom pairing in genesis.
type CreatorIndexEntry struct {
	Creator  string `json:"creator"`
	Subdenom string `json:"subdenom"`
}

// MintedLifetimeEntry represents a denom's historical lifetime minted tokens.
type MintedLifetimeEntry struct {
	Denom  string   `json:"denom"`
	Amount math.Int `json:"amount"`
}

// GenesisState defines the currencies module's genesis state.
type GenesisState struct {
	DenomPolicies   []DenomPolicy         `json:"denom_policies"`
	CreatorIndices  []CreatorIndexEntry   `json:"creator_indices"`
	MintedLifetimes []MintedLifetimeEntry `json:"minted_lifetimes"`
}

// NewGenesisState creates a new GenesisState instance.
func NewGenesisState(
	denomPolicies []DenomPolicy,
	creatorIndices []CreatorIndexEntry,
	mintedLifetimes []MintedLifetimeEntry,
) *GenesisState {
	if denomPolicies == nil {
		denomPolicies = []DenomPolicy{}
	}
	if creatorIndices == nil {
		creatorIndices = []CreatorIndexEntry{}
	}
	if mintedLifetimes == nil {
		mintedLifetimes = []MintedLifetimeEntry{}
	}
	return &GenesisState{
		DenomPolicies:   denomPolicies,
		CreatorIndices:  creatorIndices,
		MintedLifetimes: mintedLifetimes,
	}
}

// DefaultGenesis returns the default currencies genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		DenomPolicies:   []DenomPolicy{},
		CreatorIndices:  []CreatorIndexEntry{},
		MintedLifetimes: []MintedLifetimeEntry{},
	}
}

// Validate performs stateless validation of the currencies genesis state.
func (gs GenesisState) Validate() error {
	seenDenoms := make(map[string]bool)
	for _, policy := range gs.DenomPolicies {
		if policy.Denom == "" {
			return fmt.Errorf("denom policy denom cannot be empty")
		}
		if seenDenoms[policy.Denom] {
			return fmt.Errorf("duplicate denom policy found for denom: %s", policy.Denom)
		}
		seenDenoms[policy.Denom] = true

		if policy.TransferFeeBps > MaxTransferFeeBps {
			return fmt.Errorf("denom %s transfer fee bps (%d) exceeds max allowable (%d)", policy.Denom, policy.TransferFeeBps, MaxTransferFeeBps)
		}

		if policy.FeeCollectorAddress != "" {
			if _, err := sdk.AccAddressFromBech32(policy.FeeCollectorAddress); err != nil {
				return fmt.Errorf("denom %s invalid fee collector address %s: %w", policy.Denom, policy.FeeCollectorAddress, err)
			}
		}

		if policy.SupplyMode == SupplyModeCapped {
			if policy.MaxSupply.IsNil() || !policy.MaxSupply.IsPositive() {
				return fmt.Errorf("denom %s capped supply mode requires positive max_supply", policy.Denom)
			}
		}
	}

	seenPairs := make(map[string]bool)
	for _, ci := range gs.CreatorIndices {
		if ci.Creator == "" || ci.Subdenom == "" {
			return fmt.Errorf("invalid creator index entry: creator and subdenom must not be empty")
		}
		key := ci.Creator + "/" + ci.Subdenom
		if seenPairs[key] {
			return fmt.Errorf("duplicate creator index entry: %s", key)
		}
		seenPairs[key] = true
	}

	for _, ml := range gs.MintedLifetimes {
		if ml.Denom == "" {
			return fmt.Errorf("minted lifetime denom cannot be empty")
		}
		if ml.Amount.IsNil() || ml.Amount.IsNegative() {
			return fmt.Errorf("minted lifetime amount for %s must be non-negative", ml.Denom)
		}
	}

	return nil
}
