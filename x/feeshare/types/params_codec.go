package types

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/collections/codec"
)

type paramsValueCodec struct{}

// NewParamsValueCodec returns a deterministic JSON value codec for Params in collections.
func NewParamsValueCodec() codec.ValueCodec[Params] {
	return paramsValueCodec{}
}

func (paramsValueCodec) Encode(value Params) ([]byte, error) {
	return json.Marshal(value)
}

func (paramsValueCodec) Decode(b []byte) (Params, error) {
	var params Params
	if err := json.Unmarshal(b, &params); err != nil {
		return Params{}, fmt.Errorf("decode feeshare params: %w", err)
	}
	return params, nil
}

func (paramsValueCodec) EncodeJSON(value Params) ([]byte, error) {
	return json.Marshal(value)
}

func (paramsValueCodec) DecodeJSON(b []byte) (Params, error) {
	var params Params
	if err := json.Unmarshal(b, &params); err != nil {
		return Params{}, fmt.Errorf("decode feeshare params: %w", err)
	}
	return params, nil
}

func (paramsValueCodec) Stringify(value Params) string {
	return fmt.Sprintf("founder_bps=%d treasury_address=%s", value.FounderBps, value.TreasuryAddress)
}

func (paramsValueCodec) ValueType() string {
	return "feeshare/Params"
}
