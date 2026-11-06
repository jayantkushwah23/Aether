package types

import (
	"context"
	fmt "fmt"

	"google.golang.org/grpc"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ sdk.Msg = (*MsgCreateDenom)(nil)
	_ sdk.Msg = (*MsgMint)(nil)
	_ sdk.Msg = (*MsgBurn)(nil)
)

// MsgCreateDenom deploys a new factory denom and its DenomPolicy.
type MsgCreateDenom struct {
	Creator         string         `json:"creator"`
	Subdenom        string         `json:"subdenom"`
	Symbol          string         `json:"symbol"`
	Name            string         `json:"name,omitempty"`
	Decimals        uint32         `json:"decimals,omitempty"`
	SupplyMode      SupplyMode     `json:"supply_mode"`
	MaxSupply       math.Int       `json:"max_supply"`
	TransferFeeBps  uint32         `json:"transfer_fee_bps,omitempty"`
	CanBurn         bool           `json:"can_burn,omitempty"`
	CollateralDenom     string         `json:"collateral_denom,omitempty"`
	CollateralRatio     math.LegacyDec `json:"collateral_ratio,omitempty"`
	FeeCollectorAddress string         `json:"fee_collector_address,omitempty"`
}

// MsgCreateDenomResponse returns the denom that was stored.
type MsgCreateDenomResponse struct {
	Denom string `json:"denom"`
}

// MsgMint mints units of an existing factory denom to Recipient.
// Recipient defaults to Admin when empty.
type MsgMint struct {
	Admin     string   `json:"admin"`
	Denom     string   `json:"denom"`
	Amount    math.Int `json:"amount"`
	Recipient string   `json:"recipient"`
}

// MsgMintResponse returns the supply figures recorded for this mint.
type MsgMintResponse struct {
	Denom          string `json:"denom"`
	MintedLifetime string `json:"minted_lifetime"`
}

// MsgBurn burns units of an existing factory denom from Sender.
type MsgBurn struct {
	Sender string   `json:"sender"`
	Denom  string   `json:"denom"`
	Amount math.Int `json:"amount"`
}

// MsgBurnResponse returns confirmation of burned tokens.
type MsgBurnResponse struct {
	Denom  string `json:"denom"`
	Burned string `json:"burned"`
}

func (m *MsgCreateDenom) Reset()         { *m = MsgCreateDenom{} }
func (m *MsgCreateDenom) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgCreateDenom) ProtoMessage()    {}

func (m *MsgCreateDenomResponse) Reset()         { *m = MsgCreateDenomResponse{} }
func (m *MsgCreateDenomResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgCreateDenomResponse) ProtoMessage()    {}

func (m *MsgMint) Reset()         { *m = MsgMint{} }
func (m *MsgMint) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgMint) ProtoMessage()    {}

func (m *MsgMintResponse) Reset()         { *m = MsgMintResponse{} }
func (m *MsgMintResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgMintResponse) ProtoMessage()    {}

func (m *MsgBurn) Reset()         { *m = MsgBurn{} }
func (m *MsgBurn) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgBurn) ProtoMessage()    {}

func (m *MsgBurnResponse) Reset()         { *m = MsgBurnResponse{} }
func (m *MsgBurnResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgBurnResponse) ProtoMessage()    {}

// MsgServer is the server API for currencies messages.
type MsgServer interface {
	CreateDenom(ctx context.Context, msg *MsgCreateDenom) (*MsgCreateDenomResponse, error)
	Mint(ctx context.Context, msg *MsgMint) (*MsgMintResponse, error)
	Burn(ctx context.Context, msg *MsgBurn) (*MsgBurnResponse, error)
}

// RegisterMsgServer registers the MsgServer implementation with the gRPC router.
func RegisterMsgServer(s grpc.ServiceRegistrar, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

func _Msg_CreateDenom_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgCreateDenom)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).CreateDenom(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Msg/CreateDenom",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).CreateDenom(ctx, req.(*MsgCreateDenom))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_Mint_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgMint)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).Mint(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Msg/Mint",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).Mint(ctx, req.(*MsgMint))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_Burn_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgBurn)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).Burn(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cosmos.currencies.v1.Msg/Burn",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).Burn(ctx, req.(*MsgBurn))
	}
	return interceptor(ctx, in, info, handler)
}

var Msg_serviceDesc = _Msg_serviceDesc
var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "cosmos.currencies.v1.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateDenom",
			Handler:    _Msg_CreateDenom_Handler,
		},
		{
			MethodName: "Mint",
			Handler:    _Msg_Mint_Handler,
		},
		{
			MethodName: "Burn",
			Handler:    _Msg_Burn_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "cosmos/currencies/v1/tx.proto",
}
