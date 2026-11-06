package types

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/collections/codec"
	"cosmossdk.io/math"
)

type denomPolicyValueCodec struct{}

// NewDenomPolicyValueCodec returns a deterministic JSON value codec for DenomPolicy.
func NewDenomPolicyValueCodec() codec.ValueCodec[DenomPolicy] {
	return denomPolicyValueCodec{}
}

func (denomPolicyValueCodec) Encode(value DenomPolicy) ([]byte, error) {
	return encodePolicy(value)
}

func (denomPolicyValueCodec) Decode(b []byte) (DenomPolicy, error) {
	return decodePolicy(b)
}

func (denomPolicyValueCodec) EncodeJSON(value DenomPolicy) ([]byte, error) {
	return encodePolicy(value)
}

func (denomPolicyValueCodec) DecodeJSON(b []byte) (DenomPolicy, error) {
	return decodePolicy(b)
}

func (denomPolicyValueCodec) Stringify(value DenomPolicy) string {
	return fmt.Sprintf("%s mode=%d max=%s minted=%s can_burn=%t fee_bps=%d paused=%t collector=%s",
		value.Denom, value.SupplyMode, value.MaxSupply, value.MintedLifetime, value.CanBurn, value.TransferFeeBps, value.IsPaused, value.FeeCollectorAddress)
}

func (denomPolicyValueCodec) ValueType() string {
	return "currencies/DenomPolicy"
}

func encodePolicy(value DenomPolicy) ([]byte, error) {
	value = normalizePolicy(value)
	return json.Marshal(value)
}

func decodePolicy(b []byte) (DenomPolicy, error) {
	var policy DenomPolicy
	if err := json.Unmarshal(b, &policy); err != nil {
		return DenomPolicy{}, fmt.Errorf("decode denom policy: %w", err)
	}
	return normalizePolicy(policy), nil
}

func normalizePolicy(policy DenomPolicy) DenomPolicy {
	if policy.MaxSupply.IsNil() {
		policy.MaxSupply = math.ZeroInt()
	}
	if policy.MintedLifetime.IsNil() {
		policy.MintedLifetime = math.ZeroInt()
	}
	if policy.CollateralRatio.IsNil() {
		policy.CollateralRatio = math.LegacyZeroDec()
	}
	return policy
}
