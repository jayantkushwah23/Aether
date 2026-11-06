package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	storetypes "cosmossdk.io/core/store"

	"github.com/cosmos/cosmos-sdk/x/feeshare/types"
)

// Keeper defines the feeshare module keeper.
type Keeper struct {
	storeService storetypes.KVStoreService
	Schema       collections.Schema
	Params       collections.Item[types.Params]
}

// NewKeeper creates a new feeshare Keeper instance.
func NewKeeper(storeService storetypes.KVStoreService) Keeper {
	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		storeService: storeService,
		Params: collections.NewItem(
			sb,
			types.ParamsPrefix,
			"params",
			types.NewParamsValueCodec(),
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema
	return k
}

// SetParams sets the feeshare parameters in state.
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	return k.Params.Set(ctx, params)
}

// GetParams retrieves the feeshare parameters from state.
func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return types.DefaultParams(), fmt.Errorf("failed to get feeshare params: %w", err)
	}
	return params, nil
}
