package contract

import (
	"math"
	"strings"
	"testing"
)

func TestValueKindsCloneAndLimits(t *testing.T) {
	t.Parallel()

	finite, err := FloatValue(1.5)
	if err != nil {
		t.Fatalf("FloatValue: %v", err)
	}
	object, err := ObjectValue([]Field{
		{Key: "null", Value: NullValue()},
		{Key: "bool", Value: BoolValue(true)},
		{Key: "int", Value: IntValue(42)},
		{Key: "float", Value: finite},
		{Key: "string", Value: StringValue("hello")},
	})
	if err != nil {
		t.Fatalf("ObjectValue: %v", err)
	}
	list, err := ListValue([]Value{object, StringValue("tail")})
	if err != nil {
		t.Fatalf("ListValue: %v", err)
	}
	if err := list.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	values, ok := list.List()
	if !ok || len(values) != 2 {
		t.Fatalf("unexpected list: %#v, %t", values, ok)
	}
	fields, ok := values[0].Object()
	if !ok || len(fields) != 5 || fields[0].Key != "null" || fields[4].Key != "string" {
		t.Fatalf("object order changed: %#v", fields)
	}
	fields[0].Key = "mutated"
	again, _ := list.List()
	againFields, _ := again[0].Object()
	if againFields[0].Key != "null" {
		t.Fatal("Object exposed mutable internal fields")
	}

	largeFloat, err := FloatValue(1e20)
	if err != nil {
		t.Fatalf("FloatValue(1e20): %v", err)
	}
	if size, err := largeFloat.EncodedSize(); err != nil || size != 21 {
		t.Fatalf("float JSON size: got %d, err %v", size, err)
	}
	if _, err := FloatValue(math.NaN()); err == nil {
		t.Fatal("expected nonfinite float error")
	}
	if _, err := ObjectValue([]Field{{Key: "x", Value: NullValue()}, {Key: "x", Value: NullValue()}}); err == nil {
		t.Fatal("expected duplicate object key error")
	}
	tooMany := make([]Value, MaxValueItems+1)
	for index := range tooMany {
		tooMany[index] = NullValue()
	}
	if _, err := ListValue(tooMany); err == nil {
		t.Fatal("expected aggregate item limit error")
	}
	if err := StringValue(strings.Repeat("x", MaxStateValueBytes)).ValidateState(); err == nil {
		t.Fatal("expected encoded size error")
	}

	deep := NullValue()
	for range MaxValueDepth + 1 {
		deep = Value{kind: ValueList, list: []Value{deep}}
	}
	if err := deep.Validate(); err == nil {
		t.Fatal("expected depth limit error")
	}
}
