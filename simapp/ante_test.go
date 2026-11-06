package simapp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	feesharetypes "github.com/cosmos/cosmos-sdk/x/feeshare/types"
)

type mockFeeshareKeeper struct {
	params feesharetypes.Params
	err    error
}

func (m mockFeeshareKeeper) GetParams(_ context.Context) (feesharetypes.Params, error) {
	if m.err != nil {
		return feesharetypes.DefaultParams(), m.err
	}
	return m.params, nil
}

func TestFounderFeeSplitCalculation(t *testing.T) {
	treasury := sdk.AccAddress([]byte("treasury_test_addr_1"))
	mKeeper := mockFeeshareKeeper{
		params: feesharetypes.Params{
			FounderBps:      2000, // 20%
			TreasuryAddress: treasury.String(),
		},
	}

	params, err := mKeeper.GetParams(context.Background())
	require.NoError(t, err)
	require.Equal(t, uint64(2000), params.FounderBps)
	require.Equal(t, treasury.String(), params.TreasuryAddress)

	// Verify math calculation:
	feeAmt := math.NewInt(10_000)
	tenThousand := math.NewInt(10000)
	founderBpsInt := math.NewIntFromUint64(params.FounderBps)

	founderAmt := feeAmt.Mul(founderBpsInt).Quo(tenThousand)
	validatorAmt := feeAmt.Sub(founderAmt)

	require.Equal(t, math.NewInt(2000), founderAmt)
	require.Equal(t, math.NewInt(8000), validatorAmt)

	// Test dynamically changing params to 35%
	mKeeper.params.FounderBps = 3500
	params35, err := mKeeper.GetParams(context.Background())
	require.NoError(t, err)

	founderAmt35 := feeAmt.Mul(math.NewIntFromUint64(params35.FounderBps)).Quo(tenThousand)
	validatorAmt35 := feeAmt.Sub(founderAmt35)

	require.Equal(t, math.NewInt(3500), founderAmt35)
	require.Equal(t, math.NewInt(6500), validatorAmt35)
}
