package starlark

import (
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

type conversionBudget struct{ items int }

func lowerPersistentValue(value starlarkgo.Value) (contract.Value, error) {
	budget := conversionBudget{}
	return budget.lower(value, 0)
}
func (budget *conversionBudget) lower(value starlarkgo.Value, depth int) (contract.Value, error) {
	if depth > contract.MaxValueDepth {
		return contract.Value{}, fmt.Errorf("value exceeds depth %d", contract.MaxValueDepth)
	}
	switch typed := value.(type) {
	case starlarkgo.NoneType:
		return contract.NullValue(), nil
	case starlarkgo.Bool:
		return contract.BoolValue(bool(typed)), nil
	case starlarkgo.Int:
		integer, ok := typed.Int64()
		if !ok {
			return contract.Value{}, fmt.Errorf("integer is outside int64 range")
		}
		return contract.IntValue(integer), nil
	case starlarkgo.Float:
		return contract.FloatValue(float64(typed))
	case starlarkgo.String:
		return contract.StringValue(string(typed)), nil
	case *starlarkgo.List:
		budget.items += typed.Len()
		if budget.items > contract.MaxValueItems {
			return contract.Value{}, fmt.Errorf("value exceeds %d items", contract.MaxValueItems)
		}
		items := make([]contract.Value, typed.Len())
		for i := 0; i < typed.Len(); i++ {
			item, err := budget.lower(typed.Index(i), depth+1)
			if err != nil {
				return contract.Value{}, fmt.Errorf("list item %d: %w", i, err)
			}
			items[i] = item
		}
		return contract.ListValue(items)
	case starlarkgo.Tuple:
		budget.items += len(typed)
		if budget.items > contract.MaxValueItems {
			return contract.Value{}, fmt.Errorf("value exceeds %d items", contract.MaxValueItems)
		}
		items := make([]contract.Value, len(typed))
		for i, itemValue := range typed {
			item, err := budget.lower(itemValue, depth+1)
			if err != nil {
				return contract.Value{}, fmt.Errorf("tuple item %d: %w", i, err)
			}
			items[i] = item
		}
		return contract.ListValue(items)
	case *starlarkgo.Dict:
		budget.items += typed.Len()
		if budget.items > contract.MaxValueItems {
			return contract.Value{}, fmt.Errorf("value exceeds %d items", contract.MaxValueItems)
		}
		fields := make([]contract.Field, 0, typed.Len())
		for _, pair := range typed.Items() {
			key, ok := starlarkgo.AsString(pair[0])
			if !ok {
				return contract.Value{}, fmt.Errorf("object key must be string, got %s", pair[0].Type())
			}
			item, err := budget.lower(pair[1], depth+1)
			if err != nil {
				return contract.Value{}, fmt.Errorf("object field %q: %w", key, err)
			}
			fields = append(fields, contract.Field{Key: key, Value: item})
		}
		return contract.ObjectValue(fields)
	default:
		return contract.Value{}, fmt.Errorf("unsupported persistent value type %s", value.Type())
	}
}

func raisePersistentValue(value contract.Value) (starlarkgo.Value, error) {
	switch value.Kind() {
	case contract.ValueNull:
		return starlarkgo.None, nil
	case contract.ValueBool:
		item, _ := value.Bool()
		return starlarkgo.Bool(item), nil
	case contract.ValueInt:
		item, _ := value.Int()
		return starlarkgo.MakeInt64(item), nil
	case contract.ValueFloat:
		item, _ := value.Float()
		return starlarkgo.Float(item), nil
	case contract.ValueString:
		item, _ := value.String()
		return starlarkgo.String(item), nil
	case contract.ValueList:
		items, _ := value.List()
		out := make([]starlarkgo.Value, len(items))
		for index, item := range items {
			converted, err := raisePersistentValue(item)
			if err != nil {
				return nil, err
			}
			out[index] = converted
		}
		list := starlarkgo.NewList(out)
		list.Freeze()
		return list, nil
	case contract.ValueObject:
		fields, _ := value.Object()
		dict := starlarkgo.NewDict(len(fields))
		for _, field := range fields {
			converted, err := raisePersistentValue(field.Value)
			if err != nil {
				return nil, err
			}
			if err := dict.SetKey(starlarkgo.String(field.Key), converted); err != nil {
				return nil, err
			}
		}
		dict.Freeze()
		return dict, nil
	default:
		return nil, fmt.Errorf("unsupported persistent value kind %q", value.Kind())
	}
}
