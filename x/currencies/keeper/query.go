package keeper

import (
	"context"
	"errors"
	"strings"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/currencies/types"
)

var _ types.QueryServer = queryServer{}

type queryServer struct {
	k Keeper
}

// NewQueryServer returns an implementation of the Currencies QueryServer interface.
func NewQueryServer(k Keeper) types.QueryServer {
	return queryServer{k: k}
}

// DenomPolicy returns the economic policy, current supply, lifetime minted quantity, and metadata for a given denom.
func (s queryServer) DenomPolicy(ctx context.Context, req *types.QueryDenomPolicyRequest) (*types.QueryDenomPolicyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	denom := strings.TrimSpace(req.Denom)
	if denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denom cannot be empty")
	}

	policy, err := s.k.GetDenomPolicy(ctx, denom)
	if err != nil {
		if errors.Is(err, types.ErrDenomNotFound) {
			return nil, status.Errorf(codes.NotFound, "denom policy not found for %s", denom)
		}
		return nil, status.Errorf(codes.Internal, "failed to get denom policy: %v", err)
	}

	currentSupply := math.ZeroInt()
	var metadata banktypes.Metadata
	if s.k.bankKeeper != nil {
		coin := s.k.bankKeeper.GetSupply(ctx, denom)
		if !coin.Amount.IsNil() {
			currentSupply = coin.Amount
		}
		md, ok := s.k.bankKeeper.GetDenomMetaData(ctx, denom)
		if ok {
			metadata = md
		}
	}

	mintedLifetime, err := s.k.GetMintedLifetime(ctx, denom)
	if err != nil {
		mintedLifetime = math.ZeroInt()
	}

	return &types.QueryDenomPolicyResponse{
		Policy:         policy,
		CurrentSupply:  currentSupply,
		MintedLifetime: mintedLifetime,
		Metadata:       metadata,
	}, nil
}

// DenomPolicies returns a paginated list of all deployed denom policies.
func (s queryServer) DenomPolicies(ctx context.Context, req *types.QueryDenomPoliciesRequest) (*types.QueryDenomPoliciesResponse, error) {
	if req == nil {
		req = &types.QueryDenomPoliciesRequest{}
	}

	policies, pageRes, err := query.CollectionPaginate(
		ctx,
		s.k.Policies,
		req.Pagination,
		func(denom string, policy types.DenomPolicy) (types.DenomPolicy, error) {
			return policy, nil
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate denom policies: %v", err)
	}

	return &types.QueryDenomPoliciesResponse{
		Policies:   policies,
		Pagination: pageRes,
	}, nil
}

// TokenState returns comprehensive runtime status for a denom: pause state, supply headroom, and economic policy.
func (s queryServer) TokenState(ctx context.Context, req *types.QueryTokenStateRequest) (*types.QueryTokenStateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	denom := strings.TrimSpace(req.Denom)
	if denom == "" {
		return nil, status.Error(codes.InvalidArgument, "denom cannot be empty")
	}

	policy, err := s.k.GetDenomPolicy(ctx, denom)
	if err != nil {
		if errors.Is(err, types.ErrDenomNotFound) {
			return nil, status.Errorf(codes.NotFound, "denom policy not found for %s", denom)
		}
		return nil, status.Errorf(codes.Internal, "failed to get denom policy: %v", err)
	}

	currentSupply := math.ZeroInt()
	var metadata banktypes.Metadata
	if s.k.bankKeeper != nil {
		coin := s.k.bankKeeper.GetSupply(ctx, denom)
		if !coin.Amount.IsNil() {
			currentSupply = coin.Amount
		}
		md, ok := s.k.bankKeeper.GetDenomMetaData(ctx, denom)
		if ok {
			metadata = md
		}
	}

	mintedLifetime, err := s.k.GetMintedLifetime(ctx, denom)
	if err != nil {
		mintedLifetime = math.ZeroInt()
	}

	// Calculate remaining supply headroom for capped assets
	remainingSupplyCap := math.ZeroInt()
	if policy.SupplyMode == types.SupplyModeCapped && !policy.MaxSupply.IsNil() {
		if policy.CanBurn {
			// Burn restores headroom against current supply
			if policy.MaxSupply.GT(currentSupply) {
				remainingSupplyCap = policy.MaxSupply.Sub(currentSupply)
			}
		} else {
			// Headroom is strictly bounded by lifetime minted count
			if policy.MaxSupply.GT(mintedLifetime) {
				remainingSupplyCap = policy.MaxSupply.Sub(mintedLifetime)
			}
		}
	}

	return &types.QueryTokenStateResponse{
		Denom:               policy.Denom,
		IsPaused:            policy.IsPaused,
		SupplyMode:          policy.SupplyMode,
		CurrentSupply:       currentSupply,
		MaxSupply:           policy.MaxSupply,
		MintedLifetime:      mintedLifetime,
		RemainingSupplyCap:  remainingSupplyCap,
		CanBurn:             policy.CanBurn,
		CollateralDenom:     policy.CollateralDenom,
		CollateralRatio:     policy.CollateralRatio,
		TransferFeeBps:      policy.TransferFeeBps,
		FeeCollectorAddress: policy.FeeCollectorAddress,
		Metadata:            metadata,
	}, nil
}

// CreatorDenoms returns all denoms registered by a given creator address.
func (s queryServer) CreatorDenoms(ctx context.Context, req *types.QueryCreatorDenomsRequest) (*types.QueryCreatorDenomsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	creator := strings.TrimSpace(req.Creator)
	if creator == "" {
		return nil, status.Error(codes.InvalidArgument, "creator address cannot be empty")
	}

	var denoms []string
	prefixRange := collections.NewPrefixedPairRange[string, string](creator)
	err := s.k.CreatorIndex.Walk(ctx, prefixRange, func(key collections.Pair[string, string]) (bool, error) {
		subdenom := key.K2()
		denoms = append(denoms, types.FormatDenom(creator, subdenom))
		return false, nil
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to walk creator index: %v", err)
	}

	return &types.QueryCreatorDenomsResponse{
		Creator: creator,
		Denoms:  denoms,
	}, nil
}
