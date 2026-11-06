package types

import "cosmossdk.io/collections"

const (
	// ModuleName is the feeshare module name.
	ModuleName = "feeshare"

	// StoreKey is the module's KV store key.
	StoreKey = ModuleName

	// RouterKey is the message route for feeshare.
	RouterKey = ModuleName
)

var (
	// ParamsPrefix keys the feeshare Params item in store.
	ParamsPrefix = collections.NewPrefix(1)
)
