package yeva

import "fmt"

type op_code = uint8

/* [operands] stack effect */

const (
	op_pop  op_code = iota // [0] -1: a, b => a
	op_dup                 // [0] +1: a, b => a, b, b
	op_dup2                // [0] +2: a, b => a, b, a, b
	op_swap                // [0] +0: a, b => b, a

	op_begin_catch // [2] +0
	op_end_catch   // [0] +0
	op_throw       // [0] ~

	op_nihil // [0] +1
	op_false // [0] +1
	op_true  // [0] +1
	op_value // [~] +1

	op_copy_to   // [0] +0 (?)
	op_copy_from // [0] +0 (?)

	op_store_local   // [~] +0
	op_load_local    // [~] +1
	op_store_name    // [~] +0
	op_load_name     // [~] +1
	op_store_upvalue // [~] +0
	op_load_upvalue  // [~] +1

	op_closure   // [~] +1
	op_coroutine // [0] +0

	op_close_upvalue // [0] -1
	op_init_upvalue  // [~] +0

	op_structure // [0] +0

	op_define_key        // [0] -2
	op_define_key_spread // [0] -1

	op_store_key // [0] -2
	op_load_key  // [0] -1

	op_eq   // [0] -1
	op_add  // [0] -1
	op_sub  // [0] -1
	op_mul  // [0] -1
	op_pow  // [0] -1
	op_fdiv // [0] -1
	op_idiv // [0] -1
	op_mod  // [0] -1
	op_or   // [0] -1
	op_xor  // [0] -1
	op_and  // [0] -1
	op_lsh  // [0] -1
	op_rsh  // [0] -1
	op_lt   // [0] -1
	op_le   // [0] -1

	op_typeof // [0] +0
	op_not    // [0] +0
	op_rev    // [0] +0
	op_neg    // [0] +0
	op_pos    // [0] +0

	op_goto          // [2] +0
	op_goto_if_false // [2] +0
	op_goto_if_nihil // [2] +0

	op_call        // [~] ~
	op_call_spread // [~] ~
	op_return      // [0] ~
	op_suspend     // [0] +0

	op_count int = iota
)

var op_names = [...]string{
	op_pop:  "pop",
	op_dup:  "dup",
	op_dup2: "dup2",
	op_swap: "swap",

	op_begin_catch: "begin_catch",
	op_end_catch:   "end_catch",
	op_throw:       "throw",

	op_nihil: "nihil",
	op_false: "false",
	op_true:  "true",
	op_value: "value",

	op_copy_to:   "copy_to",
	op_copy_from: "copy_from",

	op_store_local:   "store local",
	op_load_local:    "load local",
	op_store_name:    "store name",
	op_load_name:     "load name",
	op_store_upvalue: "store upvalue",
	op_load_upvalue:  "load upvalue",

	op_closure:   "closure",
	op_coroutine: "coroutine",

	op_close_upvalue: "close_upvalue",
	op_init_upvalue:  "init_upvalue",

	op_structure: "structure",

	op_define_key:        "define_key",
	op_define_key_spread: "define_key_spread",

	op_store_key: "store_key",
	op_load_key:  "load_key",

	op_eq:   "eq",
	op_add:  "add",
	op_sub:  "sub",
	op_mul:  "mul",
	op_pow:  "pow",
	op_fdiv: "float div",
	op_idiv: "integer div",
	op_mod:  "mod",
	op_or:   "or",
	op_xor:  "xor",
	op_and:  "and",
	op_lsh:  "lsh",
	op_rsh:  "rsh",
	op_lt:   "lt",
	op_le:   "le",

	op_typeof: "typeof",
	op_not:    "not",
	op_rev:    "rev",
	op_neg:    "neg",
	op_pos:    "pos",

	op_goto:          "goto",
	op_goto_if_false: "goto_if_false",
	op_goto_if_nihil: "goto_if_nihil",

	op_call:        "call",
	op_call_spread: "call_spread",
	op_return:      "return",
	op_suspend:     "suspend",
}

func log_dbg_def(def *definition) {
	fmt.Println(cover(string(def.name), 30, '=') + "|")
	log_dbg_def_code(def)
	for _, d := range def.defs {
		log_dbg_def(d)
	}
}

func log_dbg_def_code(def *definition) {
	for offset := 0; offset < len(def.code); {
		offset = log_dbg_op_code(def, offset)
		fmt.Println()
	}
}

func log_dbg_op_code(def *definition, offset int) int {
	fmt.Printf("%04d", offset)
	if offset > 0 && def.lines[offset] == def.lines[offset-1] {
		fmt.Printf("   | ")
	} else {
		fmt.Printf("%4d ", def.lines[offset])
	}

	name := op_names[def.code[offset]]

	switch op := def.code[offset]; op {
	/* no */
	case op_pop, op_dup, op_dup2, op_swap,
		op_end_catch, op_throw,
		op_nihil, op_false, op_true,
		op_copy_to, op_copy_from,
		op_coroutine,
		op_close_upvalue,
		op_structure,
		op_define_key, op_define_key_spread,
		op_store_key, op_load_key,
		op_eq, op_add, op_sub, op_mul, op_pow, op_fdiv, op_idiv, op_mod,
		op_or, op_xor, op_and, op_lsh, op_rsh,
		op_lt, op_le, op_typeof, op_not, op_rev, op_neg, op_pos,
		op_return, op_suspend:
		fmt.Printf("%-20s |%16c", name, ' ')
		return offset + 1
	/* byte */
	// none
	/* constant */
	case op_value, op_store_name, op_load_name:
		idx, a := decode(def.code[offset+1:])
		fmt.Printf("%-20s |> %04d %-8v ", name, idx, def.values[idx])
		return offset + 1 + a
	/* var */
	case op_store_local, op_load_local,
		op_store_upvalue, op_load_upvalue,
		op_closure, op_init_upvalue,
		op_call, op_call_spread:
		idx, a := decode(def.code[offset+1:])
		fmt.Printf("%-20s |> %04d%10c", name, idx, ' ')
		return offset + 1 + a
	/* jump */
	case op_begin_catch, op_goto, op_goto_if_false, op_goto_if_nihil:
		jump := int(u8tou16(def.code[offset+1], def.code[offset+2]))
		fmt.Printf(
			"%-20s |> %04d >>> %04d ",
			name, offset, int16(offset+3+jump),
		)
		return offset + 3
	default:
		panic(unreachable)
	}
}
