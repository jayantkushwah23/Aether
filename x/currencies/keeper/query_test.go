package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/keeper"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

func TestQueryDenomPolicy(t *testing.T) {
	ctx, k, _, bank := setup(t)
	queryServer := keeper.NewQueryServer(k)

	creator := sdk.AccAddress([]byte("creator_test_query1")).String()
	denom := types.FormatDenom(creator, "gold")

	// Empty request
	_, err := queryServer.DenomPolicy(ctx, nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	// Non-existent denom
	_, err = queryServer.DenomPolicy(ctx, &types.QueryDenomPolicyRequest{Denom: denom})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))

	// Setup active policy
	policy := types.DenomPolicy{
		Denom:               denom,
		Creator:             creator,
		Subdenom:            "gold",
		SupplyMode:          types.SupplyModeCapped,
		MaxSupply:           math.NewInt(5_000_000),
		CanBurn:             true,
		TransferFeeBps:      200,
		FeeCollectorAddress: creator,
	}
	require.NoError(t, k.SetDenomPolicy(ctx, policy))
	require.NoError(t, k.SetMintedLifetime(ctx, denom, math.NewInt(1_000_000)))

	// Setup bank supply and metadata
	bank.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(800_000))))
	bank.SetDenomMetaData(ctx, banktypes.Metadata{
		Description: "Digital Gold Token",
		Base:        denom,
		Display:     "gold",
		Name:        "Gold Standard",
		Symbol:      "AU",
	})

	// Query DenomPolicy
	resp, err := queryServer.DenomPolicy(ctx, &types.QueryDenomPolicyRequest{Denom: denom})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, denom, resp.Policy.Denom)
	require.Equal(t, math.NewInt(800_000), resp.CurrentSupply)
	require.Equal(t, math.NewInt(1_000_000), resp.MintedLifetime)
	require.Equal(t, "Gold Standard", resp.Metadata.Name)
	require.Equal(t, "AU", resp.Metadata.Symbol)
}

func TestQueryTokenState(t *testing.T) {
	ctx, k, _, bank := setup(t)
	queryServer := keeper.NewQueryServer(k)

	creator := sdk.AccAddress([]byte("creator_test_query2")).String()
	denomCappedBurn := types.FormatDenom(creator, "cappedburn")
	denomCappedNoBurn := types.FormatDenom(creator, "cappednoburn")

	// 1. Capped with CanBurn = true: remaining cap = MaxSupply - CurrentSupply
	require.NoError(t, k.SetDenomPolicy(ctx, types.DenomPolicy{
		Denom:      denomCappedBurn,
		Creator:    creator,
		Subdenom:   "cappedburn",
		SupplyMode: types.SupplyModeCapped,
		MaxSupply:  math.NewInt(1_000_000),
		CanBurn:    true,
	}))
	bank.MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(denomCappedBurn, math.NewInt(400_000))))

	resp1, err := queryServer.TokenState(ctx, &types.QueryTokenStateRequest{Denom: denomCappedBurn})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(400_000), resp1.CurrentSupply)
	require.Equal(t, math.NewInt(600_000), resp1.RemainingSupplyCap) // 1M - 400k
	require.False(t, resp1.IsPaused)

	// Pause the token and query again
	require.NoError(t, k.SetDenomPaused(ctx, denomCappedBurn, true))
	respPaused, err := queryServer.TokenState(ctx, &types.QueryTokenStateRequest{Denom: denomCappedBurn})
	require.NoError(t, err)
	require.True(t, respPaused.IsPaused)

	// 2. Capped with CanBurn = false: remaining cap = MaxSupply - MintedLifetime
	require.NoError(t, k.SetDenomPolicy(ctx, types.DenomPolicy{
		Denom:      denomCappedNoBurn,
		Creator:    creator,
		Subdenom:   "cappednoburn",
		SupplyMode: types.SupplyModeCapped,
		MaxSupply:  math.NewInt(1_000_000),
		CanBurn:    false,
	}))
	require.NoError(t, k.SetMintedLifetime(ctx, denomCappedNoBurn, math.NewInt(700_000)))

	resp2, err := queryServer.TokenState(ctx, &types.QueryTokenStateRequest{Denom: denomCappedNoBurn})
	require.NoError(t, err)
	require.Equal(t, math.NewInt(700_000), resp2.MintedLifetime)
	require.Equal(t, math.NewInt(300_000), resp2.RemainingSupplyCap) // 1M - 700k
}

func TestQueryCreatorDenoms(t *testing.T) {
	ctx, k, _, _ := setup(t)
	queryServer := keeper.NewQueryServer(k)

	creatorA := sdk.AccAddress([]byte("creator_test_queryA")).String()
	creatorB := sdk.AccAddress([]byte("creator_test_queryB")).String()

	require.NoError(t, k.SetCreatorSubdenom(ctx, creatorA, "token1"))
	require.NoError(t, k.SetCreatorSubdenom(ctx, creatorA, "token2"))
	require.NoError(t, k.SetCreatorSubdenom(ctx, creatorB, "token3"))

	respA, err := queryServer.CreatorDenoms(ctx, &types.QueryCreatorDenomsRequest{Creator: creatorA})
	require.NoError(t, err)
	require.Len(t, respA.Denoms, 2)
	require.Contains(t, respA.Denoms, types.FormatDenom(creatorA, "token1"))
	require.Contains(t, respA.Denoms, types.FormatDenom(creatorA, "token2"))

	respB, err := queryServer.CreatorDenoms(ctx, &types.QueryCreatorDenomsRequest{Creator: creatorB})
	require.NoError(t, err)
	require.Len(t, respB.Denoms, 1)
	require.Contains(t, respB.Denoms, types.FormatDenom(creatorB, "token3"))
}

func TestQueryDenomPoliciesPagination(t *testing.T) {
	ctx, k, _, _ := setup(t)
	queryServer := keeper.NewQueryServer(k)

	creator := sdk.AccAddress([]byte("creator_test_queryP")).String()
	d1 := types.FormatDenom(creator, "alpha")
	d2 := types.FormatDenom(creator, "beta")

	require.NoError(t, k.SetDenomPolicy(ctx, types.DenomPolicy{Denom: d1, Creator: creator, Subdenom: "alpha"}))
	require.NoError(t, k.SetDenomPolicy(ctx, types.DenomPolicy{Denom: d2, Creator: creator, Subdenom: "beta"}))

	resp, err := queryServer.DenomPolicies(ctx, &types.QueryDenomPoliciesRequest{
		Pagination: &query.PageRequest{
			Limit: 10,
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Policies, 2)
}
