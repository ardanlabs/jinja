package jinja

import (
	"encoding/json"
	"fmt"
)

func decodeJSONValue(dec *json.Decoder) (Value, error) {
	token, err := dec.Token()
	if err != nil {
		return Undefined(), fmt.Errorf("decode JSON value: %w", err)
	}
	if isJSONScalar(token) {
		return FromGoValue(token), nil
	}
	delim := token.(json.Delim)
	switch delim {
	case '{':
		result := NewDict()
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return Undefined(), fmt.Errorf("decode JSON value: %w", err)
			}
			item, err := decodeJSONValue(dec)
			if err != nil {
				return Undefined(), fmt.Errorf("decode JSON value: %w", err)
			}
			result.AsDict().Set(key.(string), item)
		}
		_, err := dec.Token()
		if err != nil {
			return Undefined(), fmt.Errorf("close JSON object: %w", err)
		}
		return result, nil
	case '[':
		var items []Value
		for dec.More() {
			item, err := decodeJSONValue(dec)
			if err != nil {
				return Undefined(), fmt.Errorf("decode JSON value: %w", err)
			}
			items = append(items, item)
		}
		_, err := dec.Token()
		if err != nil {
			return Undefined(), fmt.Errorf("close JSON array: %w", err)
		}
		return NewList(items), nil
	}
	return Undefined(), fmt.Errorf("unexpected delimiter %q", delim)
}

func isJSONScalar(token any) bool {
	_, delimiter := token.(json.Delim)
	return !delimiter
}
