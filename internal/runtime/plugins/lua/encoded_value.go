package luaplugin

import "encoding/json"

type EncodedValue []byte

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
