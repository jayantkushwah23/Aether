package feeshare

import (
	"encoding/json"
	"fmt"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"

	"cosmossdk.io/core/appmodule"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/feeshare/keeper"
	"github.com/cosmos/cosmos-sdk/x/feeshare/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.HasGenesis     = AppModule{}
	_ appmodule.AppModule   = AppModule{}
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModuleBasic implements the AppModuleBasic interface for the feeshare module.
type AppModuleBasic struct{}

// NewAppModuleBasic creates a new AppModuleBasic object.
func NewAppModuleBasic() AppModuleBasic {
	return AppModuleBasic{}
}

// Name returns the feeshare module's name.
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the module's types for the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces registers the module's interface types.
func (AppModuleBasic) RegisterInterfaces(_ codectypes.InterfaceRegistry) {}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *gwruntime.ServeMux) {}

// DefaultGenesis returns the feeshare module's default genesis state as raw JSON bytes.
// Hardcodes the default genesis parameters so that 'founder_bps' initializes to 2000 (20%)
// and 'treasury_address' is set to a standard valid Bech32 address.
func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic(err)
	}
	return bz
}

// ValidateGenesis performs stateless genesis state validation for the feeshare module.
// If the parameters are invalid (e.g. founder_bps > 10000 or treasury_address is not
// a valid Bech32 address), an error is returned which prevents the chain from booting up.
func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var data types.GenesisState
	if err := json.Unmarshal(bz, &data); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return data.Validate()
}

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

// AppModule implements the AppModule interface for the feeshare module.
type AppModule struct {
	AppModuleBasic
	keeper *keeper.Keeper
}

// NewAppModule creates a new AppModule object.
func NewAppModule(k *keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         k,
	}
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the appmodule.AppModule interface.
func (AppModule) IsOnePerModuleType() {}

// DefaultGenesis returns the default genesis state as raw bytes.
func (am AppModule) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return am.AppModuleBasic.DefaultGenesis(cdc)
}

// ValidateGenesis performs genesis state validation.
func (am AppModule) ValidateGenesis(cdc codec.JSONCodec, txConfig client.TxEncodingConfig, bz json.RawMessage) error {
	return am.AppModuleBasic.ValidateGenesis(cdc, txConfig, bz)
}

// InitGenesis performs genesis initialization for the feeshare module.
// If the genesis parameters are invalid, InitGenesis panics to halt node startup.
func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, bz json.RawMessage) {
	var genesisState types.GenesisState
	if err := json.Unmarshal(bz, &genesisState); err != nil {
		panic(fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err))
	}

	if err := genesisState.Validate(); err != nil {
		panic(fmt.Errorf("invalid %s genesis parameters (preventing chain bootup): %w", types.ModuleName, err))
	}

	if am.keeper != nil {
		if err := am.keeper.SetParams(ctx, genesisState.Params); err != nil {
			panic(fmt.Sprintf("failed to store %s params in genesis: %s", types.ModuleName, err))
		}
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the feeshare module.
func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	var gs *types.GenesisState
	if am.keeper != nil {
		params, err := am.keeper.GetParams(ctx)
		if err != nil {
			gs = types.DefaultGenesis()
		} else {
			gs = types.NewGenesisState(params)
		}
	} else {
		gs = types.DefaultGenesis()
	}

	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

// ConsensusVersion implements HasConsensusVersion.
func (AppModule) ConsensusVersion() uint64 {
	return 1
}
