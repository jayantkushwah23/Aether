package types

import (
	"context"

	"cosmossdk.io/math"
	"google.golang.org/grpc"

	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// QueryDenomPolicyRequest is the request type for the Query/DenomPolicy RPC method.
type QueryDenomPolicyRequest struct {
	Denom string `json:"denom"`
}

// QueryDenomPolicyResponse is the response type for the Query/DenomPolicy RPC method.
type QueryDenomPolicyResponse struct {
	Policy         DenomPolicy        `json:"policy"`
	CurrentSupply  math.Int           `json:"current_supply"`
	MintedLifetime math.Int           `json:"minted_lifetime"`
	Metadata       banktypes.Metadata `json:"metadata"`
}

// QueryDenomPoliciesRequest is the request type for the Query/DenomPolicies RPC method.
type QueryDenomPoliciesRequest struct {
	Pagination *query.PageRequest `json:"pagination,omitempty"`
}

// QueryDenomPoliciesResponse is the response type for the Query/DenomPolicies RPC method.
type QueryDenomPoliciesResponse struct {
	Policies   []DenomPolicy       `json:"policies"`
	Pagination *query.PageResponse `json:"pagination,omitempty"`
}

// QueryTokenStateRequest is the request type for the Query/TokenState RPC method.
type QueryTokenStateRequest struct {
	Denom string `json:"denom"`
}

// QueryTokenStateResponse is the response type for the Query/TokenState RPC method.
type QueryTokenStateResponse struct {
	Denom               string             `json:"denom"`
	IsPaused            bool               `json:"is_paused"`
	SupplyMode          SupplyMode         `json:"supply_mode"`
	CurrentSupply       math.Int           `json:"current_supply"`
	MaxSupply           math.Int           `json:"max_supply"`
	MintedLifetime      math.Int           `json:"minted_lifetime"`
	RemainingSupplyCap  math.Int           `json:"remaining_supply_cap"`
	CanBurn             bool               `json:"can_burn"`
	CollateralDenom     string             `json:"collateral_denom"`
	CollateralRatio     math.LegacyDec     `json:"collateral_ratio"`
	TransferFeeBps      uint32             `json:"transfer_fee_bps"`
	FeeCollectorAddress string             `json:"fee_collector_address"`
	Metadata            banktypes.Metadata `json:"metadata"`
}

// QueryCreatorDenomsRequest is the request type for the Query/CreatorDenoms RPC method.
type QueryCreatorDenomsRequest struct {
	Creator string `json:"creator"`
}

// QueryCreatorDenomsResponse is the response type for the Query/CreatorDenoms RPC method.
type QueryCreatorDenomsResponse struct {
	Creator string   `json:"creator"`
	Denoms  []string `json:"denoms"`
}

// QueryServer is the server API for the Currencies Query service.
type QueryServer interface {
	DenomPolicy(ctx context.Context, req *QueryDenomPolicyRequest) (*QueryDenomPolicyResponse, error)
	DenomPolicies(ctx context.Context, req *QueryDenomPoliciesRequest) (*QueryDenomPoliciesResponse, error)
	TokenState(ctx context.Context, req *QueryTokenStateRequest) (*QueryTokenStateResponse, error)
	CreatorDenoms(ctx context.Context, req *QueryCreatorDenomsRequest) (*QueryCreatorDenomsResponse, error)
}

// RegisterQueryServer registers the QueryServer to the service registrar.
func RegisterQueryServer(s grpc.ServiceRegistrar, srv QueryServer) {
	s.RegisterService(&Query_serviceDesc, srv)
}

func _Query_DenomPolicy_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryDenomPolicyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).DenomPolicy(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Query/DenomPolicy",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).DenomPolicy(ctx, req.(*QueryDenomPolicyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_DenomPolicies_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryDenomPoliciesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).DenomPolicies(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Query/DenomPolicies",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).DenomPolicies(ctx, req.(*QueryDenomPoliciesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_TokenState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryTokenStateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).TokenState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Query/TokenState",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).TokenState(ctx, req.(*QueryTokenStateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_CreatorDenoms_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryCreatorDenomsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).CreatorDenoms(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Query/CreatorDenoms",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).CreatorDenoms(ctx, req.(*QueryCreatorDenomsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var (
	_Query_serviceDesc = grpc.ServiceDesc{
		ServiceName: "cosmos.currencies.v1.Query",
		HandlerType: (*QueryServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "DenomPolicy",
				Handler:    _Query_DenomPolicy_Handler,
			},
			{
				MethodName: "DenomPolicies",
				Handler:    _Query_DenomPolicies_Handler,
			},
			{
				MethodName: "TokenState",
				Handler:    _Query_TokenState_Handler,
			},
			{
				MethodName: "CreatorDenoms",
				Handler:    _Query_CreatorDenoms_Handler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "cosmos/currencies/v1/query.proto",
	}

	// Query_serviceDesc is the exported descriptor for gRPC query service registration.
	Query_serviceDesc = _Query_serviceDesc
)
