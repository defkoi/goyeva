package yeva

import (
	"errors"
	"reflect"
)

func ToValue(v any) Value {
	switch v := v.(type) {
	case bool:
		return Boolean(v)

	case int:
		return Number(v)
	case float64:
		return Number(v)

	case string:
		return String(v)

	case reflect.Value:
		switch v.Kind() {
		case reflect.Bool:
			return Boolean(v.Bool())
		case reflect.Int:
			return Number(v.Int())
		case reflect.Float64:
			return Number(v.Float())
		case reflect.String:
			return String(v.String())
		default:
			return Nihil{}
		}

	default:
		return Nihil{}
	}
}

func WrapFunction(fn any) (Value, error) {
	v := reflect.ValueOf(fn)
	t := v.Type()

	if t.Kind() != reflect.Func {
		return nil, errors.New("not a function")
	}

	numIn := t.NumIn()
	numOut := t.NumOut()

	inKinds := make([]reflect.Kind, numIn)
	for i := range numIn {
		k := t.In(i).Kind()
		if !map_has(export, k) {
			return nil, errors.New("invalid parameter")
		}
		inKinds[i] = k
	}

	return &Native{func(e *Engine, args []Value) (Value, err_value) {
		if len(args) < numIn {
			return nil, Stringf(
				"expected %d agruments, got %d",
				numIn, len(args),
			)
		}

		goArgs := make([]reflect.Value, numIn)
		for i, k := range inKinds {
			goArgs[i] = export[k](args[i])
		}

		result := v.Call(goArgs)

		switch numOut {
		case 0:
			return Nihil{}, nil
		case 1:
			return ToValue(result[0]), nil
		default:
			return ToValue(result[0]), ToValue(result[1])
		}
	}}, nil
}

var export = map[reflect.Kind]func(Value) reflect.Value{
	reflect.Bool: func(v Value) reflect.Value {
		return reflect.ValueOf(to_boolean(v))
	},

	reflect.Int: func(v Value) reflect.Value {
		return reflect.ValueOf(int(ToFloat(v)))
	},
	reflect.Float64: func(v Value) reflect.Value {
		return reflect.ValueOf(ToFloat(v))
	},

	reflect.String: func(v Value) reflect.Value {
		return reflect.ValueOf(v.String())
	},
}
