package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

// InitGenesis initializes the currencies module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState *types.GenesisState) error {
	if genState == nil {
		return nil
	}
	if err := genState.Validate(); err != nil {
		return fmt.Errorf("invalid currencies genesis state: %w", err)
	}

	for _, policy := range genState.DenomPolicies {
		if err := k.SetDenomPolicy(ctx, policy); err != nil {
			return fmt.Errorf("failed to set denom policy %s: %w", policy.Denom, err)
		}
		if policy.Creator != "" && policy.Subdenom != "" {
			if err := k.SetCreatorSubdenom(ctx, policy.Creator, policy.Subdenom); err != nil {
				return fmt.Errorf("failed to set creator index for %s/%s: %w", policy.Creator, policy.Subdenom, err)
			}
		}
	}

	for _, ci := range genState.CreatorIndices {
		if err := k.SetCreatorSubdenom(ctx, ci.Creator, ci.Subdenom); err != nil {
			return fmt.Errorf("failed to set creator index entry for %s/%s: %w", ci.Creator, ci.Subdenom, err)
		}
	}

	for _, ml := range genState.MintedLifetimes {
		if err := k.SetMintedLifetime(ctx, ml.Denom, ml.Amount); err != nil {
			return fmt.Errorf("failed to set minted lifetime for %s: %w", ml.Denom, err)
		}
	}

	return nil
}

// ExportGenesis returns the currencies module's exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	genState := types.DefaultGenesis()

	// 1. Export all DenomPolicies
	err := k.Policies.Walk(ctx, nil, func(denom string, policy types.DenomPolicy) (bool, error) {
		genState.DenomPolicies = append(genState.DenomPolicies, policy)
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export denom policies: %w", err)
	}

	// 2. Export CreatorIndex
	err = k.CreatorIndex.Walk(ctx, nil, func(key collections.Pair[string, string]) (bool, error) {
		genState.CreatorIndices = append(genState.CreatorIndices, types.CreatorIndexEntry{
			Creator:  key.K1(),
			Subdenom: key.K2(),
		})
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export creator index: %w", err)
	}

	// 3. Export MintedLifetimes
	err = k.MintedLifetime.Walk(ctx, nil, func(denom string, amount math.Int) (bool, error) {
		genState.MintedLifetimes = append(genState.MintedLifetimes, types.MintedLifetimeEntry{
			Denom:  denom,
			Amount: amount,
		})
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export minted lifetimes: %w", err)
	}

	return genState, nil
}
