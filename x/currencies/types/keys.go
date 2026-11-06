package types

import "cosmossdk.io/collections"

const (
	// ModuleName is the x/currencies module name.
	ModuleName = "currencies"

	// StoreKey is the module's KV store key.
	StoreKey = ModuleName

	// RouterKey is the message route for currencies messages.
	RouterKey = ModuleName

	// DenomPrefix is the namespace for denoms created by MsgCreateDenom.
	DenomPrefix = "factory"
)

var (
	// PoliciesPrefix keys DenomPolicy by full denom.
	PoliciesPrefix = collections.NewPrefix(1)

	// CreatorIndexPrefix keys (creator, subdenom) pairs in CreatorIndex.
	CreatorIndexPrefix = collections.NewPrefix(2)

	// MintedLifetimePrefix keys cumulative minted lifetime per denom in a separate register.
	MintedLifetimePrefix = collections.NewPrefix(3)
)
