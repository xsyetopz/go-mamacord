package luaplugin

import "encoding/json"

type EncodedValue []byte

type PayloadOptions struct {
	values map[string]any
}

func NewPayloadOptions(values map[string]any) PayloadOptions {
	if len(values) == 0 {
		return PayloadOptions{}
	}
	return PayloadOptions{values: values}
}

func (o PayloadOptions) Map() map[string]any {
	return o.values
}

func (o PayloadOptions) Lookup(key string) (any, bool) {
	if len(o.values) == 0 {
		return nil, false
	}
	value, ok := o.values[key]
	return value, ok
}

func (o PayloadOptions) Range(fn func(key string, value any) bool) {
	for key, value := range o.values {
		if !fn(key, value) {
			return
		}
	}
}

func EncodeValue(value any) (EncodedValue, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return EncodedValue(raw), nil
}

func (v EncodedValue) Decode() (any, error) {
	if len(v) == 0 {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal(v, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
