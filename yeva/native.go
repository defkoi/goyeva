package yeva

import (
	"fmt"
	"iter"
	"time"
)

func native_print(e *engine, args []Value) (result Value, throw err_value) {
	for i, arg := range args {
		switch arg := arg.(type) {
		case *Structure:
			fmt.Print(structure_format(arg))
		default:
			fmt.Print(arg)
		}
		if i != len(args)-1 {
			fmt.Print(" ")
		}
	}
	fmt.Println()
	return Nihil{}, nil
}

func native_clock(e *engine, args []Value) (result Value, throw err_value) {
	return Number(float64(time.Now().UnixNano()) / float64(time.Second)), nil
}

func native_pairs(e *engine, args []Value) (result Value, throw err_value) {
	const (
		key_key   = String("key")
		key_value = String("value")
	)
	if len(args) < 1 || !is[*Structure](args[0]) {
		return nil, String("structure expected")
	}
	s := args[0].(*Structure)
	next, stop := iter.Pull2(s.pairs())
	f := func(y *engine, args []Value) (result Value, throw err_value) {
		k, v, ok := next()
		if !ok {
			stop()
			return Nihil{}, nil
		}
		r := new_structure(Nihil{})
		r.store(key_key, k)
		r.store(key_value, v)
		return r, nil
	}
	return &Native{f}, nil
}
