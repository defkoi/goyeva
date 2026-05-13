package yeva

import (
	"fmt"
	"math"
	"strconv"
)

const (
	Name = "yeva"
)

var (
	Version = version{0, 0, 0}
)

func New() *Engine {
	e := &Engine{
		globals: map[String]Value{
			"print": NewNative(native_print),
			"clock": NewNative(native_clock),
			"pairs": NewNative(native_pairs),
		},
		call_stack: make([]call_frame, 0, call_stack_limit),
		stack:      make([]Value, 0, stack_init_cap),
	}
	assert, err := e.Interpret(embed)
	if err != nil {
		panic(err)
	}
	e.globals["assert"] = assert
	return e
}

func (e *Engine) Interpret(src []byte) (Value, error) {
	c := new_compiler(src)
	def, err := c.compile()
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}
	if dbg_code {
		log_dbg_def(def)
	}
	cls := &Closure{def: def}
	if dbg_exe {
		fmt.Println(cover("@execution", 30, '=') + "|")
	}
	e.push(cls)
	cls.call(e, 0)
	if v, err := e.execute(); err != nil {
		return nil, fmt.Errorf("runtime error: %v", err)
	} else {
		return v, nil
	}
}

func (e *Engine) Call(c Callable, a ...Value) (Value, error) {
	e.push(c)
	for _, a := range a {
		e.push(a)
	}
	e.call_value(c, len(a))
	if v, err := e.execute(); err != nil {
		return nil, fmt.Errorf("runtime error: %v", err)
	} else {
		return v, nil
	}
}

func (e *Engine) StoreGlobal(name string, value Value) {
	e.globals[String(name)] = value
}

func (e *Engine) LoadGlobal(name string) Value {
	return e.globals[String(name)]
}

func ToBool(v Value) bool {
	return bool(to_boolean(v))
}

func ToFloat(v Value) float64 {
	switch v := v.(type) {
	case Number:
		return float64(v)
	case String:
		parsed, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return math.NaN()
		}
		return parsed
	default:
		return math.NaN()
	}
}
