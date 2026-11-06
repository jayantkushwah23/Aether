package keeper

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

// Keeper stores per-denom economic policy for factory tokens.
type Keeper struct {
	storeService storetypes.KVStoreService
	bankKeeper   types.BankKeeper

	Schema         collections.Schema
	Policies       collections.Map[string, types.DenomPolicy]
	CreatorIndex   collections.KeySet[collections.Pair[string, string]]
	MintedLifetime collections.Map[string, math.Int]
}

// NewKeeper constructs the currencies keeper.
func NewKeeper(storeService storetypes.KVStoreService, bankKeeper types.BankKeeper) Keeper {
	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		storeService: storeService,
		bankKeeper:   bankKeeper,
		Policies: collections.NewMap(
			sb,
			types.PoliciesPrefix,
			"denom_policies",
			collections.StringKey,
			types.NewDenomPolicyValueCodec(),
		),
		CreatorIndex: collections.NewKeySet(
			sb,
			types.CreatorIndexPrefix,
			"creator_index",
			collections.PairKeyCodec(collections.StringKey, collections.StringKey),
		),
		MintedLifetime: collections.NewMap(
			sb,
			types.MintedLifetimePrefix,
			"minted_lifetime",
			collections.StringKey,
			sdk.IntValue,
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema
	return k
}

// BankKeeper returns the underlying bank keeper interface.
func (k Keeper) BankKeeper() types.BankKeeper {
	return k.bankKeeper
}

// SetDenomPolicy writes the policy for denom.
func (k Keeper) SetDenomPolicy(ctx context.Context, policy types.DenomPolicy) error {
	if policy.Denom == "" {
		return fmt.Errorf("denom policy requires a denom")
	}
	return k.Policies.Set(ctx, policy.Denom, policy)
}

// GetDenomPolicy returns the policy for denom.
func (k Keeper) GetDenomPolicy(ctx context.Context, denom string) (types.DenomPolicy, error) {
	policy, err := k.Policies.Get(ctx, denom)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.DenomPolicy{}, fmt.Errorf("denom %s: %w", denom, types.ErrDenomNotFound)
		}
		return types.DenomPolicy{}, err
	}
	return policy, nil
}

// HasDenomPolicy reports whether a policy is stored for denom.
func (k Keeper) HasDenomPolicy(ctx context.Context, denom string) (bool, error) {
	return k.Policies.Has(ctx, denom)
}

// HasCreatorSubdenom reports whether creator has already deployed subdenom in CreatorIndex.
func (k Keeper) HasCreatorSubdenom(ctx context.Context, creator, subdenom string) (bool, error) {
	return k.CreatorIndex.Has(ctx, collections.Join(creator, subdenom))
}

// SetCreatorSubdenom records that creator has deployed subdenom in CreatorIndex.
func (k Keeper) SetCreatorSubdenom(ctx context.Context, creator, subdenom string) error {
	return k.CreatorIndex.Set(ctx, collections.Join(creator, subdenom))
}

// GetMintedLifetime returns the cumulative minted lifetime for denom from the separate collections register.
// If not yet set in the collections register, it falls back to policy.MintedLifetime.
func (k Keeper) GetMintedLifetime(ctx context.Context, denom string) (math.Int, error) {
	lifetime, err := k.MintedLifetime.Get(ctx, denom)
	if err == nil {
		return lifetime, nil
	}
	if !errors.Is(err, collections.ErrNotFound) {
		return math.ZeroInt(), err
	}
	// Fallback to policy.MintedLifetime if stored
	policy, pErr := k.GetDenomPolicy(ctx, denom)
	if pErr == nil && !policy.MintedLifetime.IsNil() {
		return policy.MintedLifetime, nil
	}
	return math.ZeroInt(), nil
}

// SetMintedLifetime updates the cumulative minted lifetime for denom in the separate collections register.
func (k Keeper) SetMintedLifetime(ctx context.Context, denom string, amount math.Int) error {
	return k.MintedLifetime.Set(ctx, denom, amount)
}

// SetDenomPaused updates the pause state for an existing denom policy.
func (k Keeper) SetDenomPaused(ctx context.Context, denom string, isPaused bool) error {
	policy, err := k.GetDenomPolicy(ctx, denom)
	if err != nil {
		return err
	}
	policy.IsPaused = isPaused
	return k.SetDenomPolicy(ctx, policy)
}
