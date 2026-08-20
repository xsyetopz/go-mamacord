package jsonobject

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func RejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder, "manifest"); err != nil {
		return err
	}
	if token, err := decoder.Token(); err == nil {
		return fmt.Errorf("manifest has trailing token %v", token)
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse Starlark manifest: %w", err)
	}
	return nil
}
func consumeJSONValue(decoder *json.Decoder, location string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("parse Starlark manifest: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("manifest object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%s has duplicate field %q", location, key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder, location+"."+key); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("manifest object is not closed")
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := consumeJSONValue(decoder, fmt.Sprintf("%s[%d]", location, index)); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("manifest array is not closed")
		}
	default:
		return errors.New("manifest contains invalid delimiter")
	}
	return nil
}
