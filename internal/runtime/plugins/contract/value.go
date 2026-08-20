package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"
)

const (
	MaxValueDepth           = 16
	MaxValueItems           = 500
	MaxStateValueBytes      = 16 * 1024
	MaxInvocationValueBytes = 64 * 1024
)

type ValueKind string

const (
	ValueNull   ValueKind = "null"
	ValueBool   ValueKind = "bool"
	ValueInt    ValueKind = "int"
	ValueFloat  ValueKind = "float"
	ValueString ValueKind = "string"
	ValueList   ValueKind = "list"
	ValueObject ValueKind = "object"
)

type Field struct {
	Key   string
	Value Value
}

type Value struct {
	kind    ValueKind
	boolean bool
	integer int64
	number  float64
	text    string
	list    []Value
	object  []Field
}

func NullValue() Value {
	return Value{kind: ValueNull}
}
func BoolValue(value bool) Value {
	return Value{kind: ValueBool, boolean: value}
}
func IntValue(value int64) Value {
	return Value{kind: ValueInt, integer: value}
}

func FloatValue(value float64) (Value, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Value{}, errors.New("float value must be finite")
	}
	return Value{kind: ValueFloat, number: value}, nil
}

func StringValue(value string) Value {
	return Value{kind: ValueString, text: value}
}

func ListValue(values []Value) (Value, error) {
	out := Value{kind: ValueList, list: cloneValues(values)}
	if err := out.Validate(); err != nil {
		return Value{}, err
	}
	return out, nil
}

func ObjectValue(fields []Field) (Value, error) {
	copyFields := cloneFields(fields)
	seen := make(map[string]struct{}, len(copyFields))
	for _, field := range copyFields {
		if field.Key == "" {
			return Value{}, errors.New("object key cannot be empty")
		}
		if _, exists := seen[field.Key]; exists {
			return Value{}, fmt.Errorf("duplicate object key %q", field.Key)
		}
		seen[field.Key] = struct{}{}
	}
	out := Value{kind: ValueObject, object: copyFields}
	if err := out.Validate(); err != nil {
		return Value{}, err
	}
	return out, nil
}

func (v Value) Kind() ValueKind {
	return v.kind
}
func (v Value) Bool() (bool, bool) {
	return v.boolean, v.kind == ValueBool
}
func (v Value) Int() (int64, bool) {
	return v.integer, v.kind == ValueInt
}
func (v Value) Float() (float64, bool) {
	return v.number, v.kind == ValueFloat
}
func (v Value) String() (string, bool) {
	return v.text, v.kind == ValueString
}

func (v Value) List() ([]Value, bool) {
	if v.kind != ValueList {
		return nil, false
	}
	return cloneValues(v.list), true
}

func (v Value) Object() ([]Field, bool) {
	if v.kind != ValueObject {
		return nil, false
	}
	return cloneFields(v.object), true
}

func (v Value) Clone() Value {
	out := v
	out.list = cloneValues(v.list)
	out.object = cloneFields(v.object)
	return out
}

func (v Value) Validate() error {
	items := 0
	_, err := v.validate(0, &items)
	return err
}

func (v Value) ValidateState() error {
	size, err := v.EncodedSize()
	if err != nil {
		return err
	}
	if size > MaxStateValueBytes {
		return fmt.Errorf("value exceeds %d-byte encoded limit", MaxStateValueBytes)
	}
	return nil
}

func (v Value) EncodedSize() (int, error) {
	items := 0
	return v.validate(0, &items)
}

func (v Value) validate(depth int, items *int) (int, error) {
	if depth > MaxValueDepth {
		return 0, fmt.Errorf("value exceeds maximum depth %d", MaxValueDepth)
	}
	switch v.kind {
	case ValueNull:
		return len("null"), nil
	case ValueBool:
		if v.boolean {
			return len("true"), nil
		}
		return len("false"), nil
	case ValueInt:
		return len(strconv.FormatInt(v.integer, 10)), nil
	case ValueFloat:
		if math.IsNaN(v.number) || math.IsInf(v.number, 0) {
			return 0, errors.New("float value must be finite")
		}
		encoded, err := json.Marshal(v.number)
		if err != nil {
			return 0, fmt.Errorf("encode float value: %w", err)
		}
		return len(encoded), nil
	case ValueString:
		if !utf8.ValidString(v.text) {
			return 0, errors.New("string value must be valid UTF-8")
		}
		encoded, err := json.Marshal(v.text)
		if err != nil {
			return 0, fmt.Errorf("encode string value: %w", err)
		}
		return len(encoded), nil
	case ValueList:
		*items += len(v.list)
		if *items > MaxValueItems {
			return 0, fmt.Errorf("value exceeds aggregate item limit %d", MaxValueItems)
		}
		size := 2
		for index, item := range v.list {
			itemSize, err := item.validate(depth+1, items)
			if err != nil {
				return 0, fmt.Errorf("list item %d: %w", index, err)
			}
			if index > 0 {
				size++
			}
			size += itemSize
		}
		return size, nil
	case ValueObject:
		*items += len(v.object)
		if *items > MaxValueItems {
			return 0, fmt.Errorf("value exceeds aggregate item limit %d", MaxValueItems)
		}
		seen := make(map[string]struct{}, len(v.object))
		size := 2
		for index, field := range v.object {
			if field.Key == "" {
				return 0, errors.New("object key cannot be empty")
			}
			if !utf8.ValidString(field.Key) {
				return 0, errors.New("object key must be valid UTF-8")
			}
			if _, exists := seen[field.Key]; exists {
				return 0, fmt.Errorf("duplicate object key %q", field.Key)
			}
			seen[field.Key] = struct{}{}
			fieldSize, err := field.Value.validate(depth+1, items)
			if err != nil {
				return 0, fmt.Errorf("object field %q: %w", field.Key, err)
			}
			if index > 0 {
				size++
			}
			encodedKey, err := json.Marshal(field.Key)
			if err != nil {
				return 0, fmt.Errorf("encode object key %q: %w", field.Key, err)
			}
			size += len(encodedKey) + 1 + fieldSize
		}
		return size, nil
	default:
		return 0, fmt.Errorf("unsupported value kind %q", v.kind)
	}
}

func cloneValues(values []Value) []Value {
	if len(values) == 0 {
		return nil
	}
	out := make([]Value, len(values))
	for index := range values {
		out[index] = values[index].Clone()
	}
	return out
}

func cloneFields(fields []Field) []Field {
	if len(fields) == 0 {
		return nil
	}
	out := make([]Field, len(fields))
	for index, field := range fields {
		out[index] = Field{Key: field.Key, Value: field.Value.Clone()}
	}
	return out
}
