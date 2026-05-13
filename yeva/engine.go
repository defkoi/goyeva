package yeva

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strings"
)

type call_frame struct {
	closure *Closure
	pc      int
	slots   int
}

func (cf *call_frame) read_var() int {
	v, a := decode(cf.closure.def.code[cf.pc:])
	cf.pc += a
	return v
}

func (cf *call_frame) read_byte() uint8 {
	cf.pc++
	return cf.closure.def.code[cf.pc-1]
}

func (cf *call_frame) read_short() uint16 {
	return u8tou16(cf.read_byte(), cf.read_byte())
}

func (cf *call_frame) read_long() uint32 {
	return u8tou32(
		cf.read_byte(),
		cf.read_byte(),
		cf.read_byte(),
		cf.read_byte(),
	)
}

func (cf *call_frame) read_value() Value {
	return cf.closure.def.values[cf.read_var()]
}

func (cf *call_frame) read_definition() *definition {
	return cf.closure.def.defs[cf.read_var()]
}

func (cf *call_frame) read_string() String {
	return cf.read_value().(String)
}

type catch_handler struct {
	recover        int
	stack_len      int
	call_stack_len int
}

type Engine struct {
	globals        map[String]Value
	call_stack     []call_frame
	stack          []Value
	catch_handlers []catch_handler
	temp           Value
	open_upv       *linked_node[*upvalue]
	status         coros
}

func (e *Engine) add_open_upval(abs_loc int) *upvalue {
	var prev *linked_node[*upvalue] = nil
	curr := e.open_upv
	for curr != nil && curr.value.abs_loc > abs_loc {
		prev = curr
		curr = curr.next
	}
	if curr != nil && curr.value.abs_loc == abs_loc {
		return curr.value
	}
	new_upvalue := &upvalue{
		abs_loc: abs_loc,
		ref:     &e.stack,
	}
	new_node := &linked_node[*upvalue]{value: new_upvalue, next: curr}
	if prev != nil {
		prev.next = new_node
	} else {
		e.open_upv = new_node
	}
	return new_upvalue
}

func (e *Engine) close_upvals(loc_of_last int) {
	var save *linked_node[*upvalue] = nil
	var cur_save *linked_node[*upvalue] = save
	for e.open_upv != nil && e.open_upv.value.abs_loc >= loc_of_last {
		if e.open_upv.value.is_init {
			e.open_upv.value.close()
		} else if cur_save != nil {
			cur_save.next = e.open_upv
			cur_save = cur_save.next
		} else {
			save = e.open_upv
		}
		e.open_upv = e.open_upv.next
	}
	if save != nil {
		cur_save.next = e.open_upv
		e.open_upv = save
	}
}

func (e *Engine) bind_upvalue2(upval *upvalue, info upval2_info) {
	switch info := info.(type) {
	case upval2_name_info:
		upval.up2 = up2(info)
	case upval2_open_info:
		/* if raw index */
		fr := e.call_stack[len(e.call_stack)-1-info.back]
		upval.up2 = up2(fr.slots + info.loc)
		/* elif upvalue index */
		upval.up2 = up2(e.current_frame().closure.upvals[info.loc].abs_loc)
	case nil:
		upval.is_init = true
	default:
		panic(unreachable)
	}
}

func (e *Engine) call_value(callee Value, argc int) err_value {
	switch callee := callee.(type) {
	case Callable:
		return callee.call(e, argc)
	default:
		return Stringf("attempt to call %s", callee.TypeOf())
	}
}

func (e *Engine) balance_args(argc int, paramc int, vararg bool) {
	var varg *Structure
	if vararg {
		varg = NewStructure(Nihil{})
	}
	if argc <= paramc {
		for range paramc - argc {
			e.push(Nihil{})
		}
	} else {
		shift := argc - paramc
		if vararg {
			for i := range shift {
				varg.Store(Number(shift-1-i), e.pop())
			}
		} else {
			slice_cut(&e.stack, -shift)
		}
	}
	if vararg {
		e.push(varg)
	}
}

func (e *Engine) store_local(cf *call_frame, idx int, v Value) {
	e.stack[cf.slots+idx] = v
}

func (e *Engine) load_local(cf *call_frame, idx int) Value {
	return e.stack[cf.slots+idx]
}

func (e *Engine) store_global(name String, v Value) err_value {
	// _, ok := e.globals[name]
	// if !ok {
	// 	return Stringf("variable '%s' is not defined", name)
	// }
	e.globals[name] = v
	return nil
}

func (e *Engine) load_global(name String) (Value, err_value) {
	v, ok := e.globals[name]
	if !ok {
		return Nihil{}, nil
		// return nil, Stringf("variable '%s' is not defined", name)
	}
	return v, nil
}

func (e *Engine) push(v Value) {
	slice_push(&e.stack, v)
}

func (e *Engine) pushf(format string, a ...any) {
	e.push(String(fmt.Sprintf(format, a...)))
}

func (e *Engine) pop() (v Value) {
	return slice_pop(&e.stack)
}

func (e *Engine) peek1() (v Value) {
	return e.stack[len(e.stack)-1]
}

func (e *Engine) peek2() (v Value) {
	return e.stack[len(e.stack)-2]
}

func (e *Engine) peek(i int) (v Value) {
	return e.stack[len(e.stack)+i]
}

func (e *Engine) current_frame() *call_frame {
	return slice_last(e.call_stack)
}

func (e *Engine) unwind(fr **call_frame) bool {
	if len(e.catch_handlers) != 0 {
		handler := slice_pop(&e.catch_handlers)
		e.stack = e.stack[:handler.stack_len]
		e.call_stack = e.call_stack[:handler.call_stack_len]
		*fr = e.current_frame()
		(*fr).pc = handler.recover
		return true
	}
	return false
}

func (e *Engine) uncaught(v Value) error {
	var err_builder strings.Builder
	fmt.Fprintf(&err_builder, "uncaught: %s\n", v)
	fmt.Fprintf(&err_builder, "\tstack trace:")
	for i := len(e.call_stack) - 1; i >= 0; i-- {
		frame := &e.call_stack[i]
		def := frame.closure.def
		line := def.lines[frame.pc-1]
		fmt.Fprintf(&err_builder, "\n\t\tln %d: in function %s", line, def.name)
	}
	return errors.New(err_builder.String())
}

func (e *Engine) execute() (_ Value, err error) {
	defer catch(func(e fatal_error) { err = e })

	e.status = coros_running
	fr := e.current_frame()
	for {
		if dbg_exe {
			log_dbg_op_code(fr.closure.def, fr.pc)
			for _, v := range e.stack {
				fmt.Printf("[%v]", v)
			}
			fmt.Println()
		}
		switch op := fr.read_byte(); op {
		case op_pop:
			e.pop()
		case op_dup:
			e.push(e.peek1())
		case op_dup2:
			e.push(e.peek2())
			e.push(e.peek2())
		case op_swap:
			e.stack[len(e.stack)-1],
				e.stack[len(e.stack)-2] =
				e.peek2(),
				e.peek1()
		case op_begin_catch:
			rcvr := int(int16(fr.read_short()))
			slice_push(&e.catch_handlers, catch_handler{
				recover:        fr.pc + rcvr,
				stack_len:      len(e.stack),
				call_stack_len: len(e.call_stack),
			})
		case op_end_catch:
			slice_pop(&e.catch_handlers)
			v := e.pop()
			r := NewStructure(Nihil{})
			r.Store(key_result, v)
			e.push(r)
		case op_throw:
			goto unwind
		case op_nihil:
			e.push(Nihil{})
		case op_false:
			e.push(Boolean(false))
		case op_true:
			e.push(Boolean(true))
		case op_value:
			e.push(fr.read_value())
		case op_copy_to:
			e.temp = e.peek1()
		case op_copy_from:
			e.stack[len(e.stack)-1] = e.temp
		case op_store_local:
			e.store_local(fr, fr.read_var(), e.peek1())
		case op_load_local:
			e.push(e.load_local(fr, fr.read_var()))
		case op_store_name:
			if err := e.store_global(fr.read_string(), e.peek1()); err != nil {
				e.push(err)
				goto unwind
			}
		case op_load_name:
			v, err := e.load_global(fr.read_string())
			if err != nil {
				e.push(err)
				goto unwind
			}
			e.push(v)
		case op_store_upvalue:
			err := fr.closure.upvals[fr.read_var()].store(e, e.peek1())
			if err != nil {
				e.push(err)
				goto unwind
			}
		case op_load_upvalue:
			v, err := fr.closure.upvals[fr.read_var()].load(e)
			if err != nil {
				e.push(err)
				goto unwind
			}
			e.push(v)
		case op_closure:
			e.push(new_closure(fr.read_definition(), e, fr))
		case op_coroutine:
			e.push(new_coroutine(e.pop().(*Closure)))
		case op_close_upvalue:
			e.close_upvals(len(e.stack) - 1)
			e.pop()
		case op_init_upvalue:
			fr.closure.upvals[fr.read_var()].is_init = true
		case op_structure:
			v := e.pop()
			if p, ok := v.(StructProto); !ok {
				e.pushf("prototype is %s", v.TypeOf())
				goto unwind

			} else {
				e.push(NewStructure(p))
			}
		case op_define_key:
			v := e.pop()
			k := e.pop()
			s := e.peek1().(*Structure)
			if err := s.Store(k, v); err != nil {
				e.push(err)
				goto unwind
			}
		case op_define_key_spread:
			v := e.pop()
			s := e.peek1().(*Structure)
			vs, ok := v.(*Structure)
			if !ok {
				e.pushf("attempt to spread %s", v.TypeOf())
				goto unwind
			}
			s.Merge(vs)
		case op_store_key:
			v := e.pop()
			k := e.pop()
			to := e.pop()
			s, ok := to.(*Structure)
			if !ok {
				e.pushf("attempt to store key to %s", to.TypeOf())
				goto unwind
			}
			if err := s.Store(k, v); err != nil {
				e.push(err)
				goto unwind
			}
			e.push(v)
		case op_load_key:
			k := e.pop()
			to := e.pop()
			s, ok := to.(*Structure)
			if !ok {
				e.pushf("attempt to load key from %s", to.TypeOf())
				goto unwind
			}
			e.push(s.Load(k))
		case op_typeof:
			e.push(e.pop().TypeOf())
		case op_not:
			e.push(!to_boolean(e.pop()))
		case op_rev, op_neg, op_pos:
			v := e.pop()
			if n, ok := v.(Number); ok {
				e.push(un_num_ops[op-op_rev](n))
			} else {
				e.pushf("attempt to %s %s", op_names[op], v.TypeOf())
				goto unwind
			}
		case op_eq:
			v2 := e.pop()
			v1 := e.pop()
			e.push(Boolean(v1 == v2))
		case op_add, op_sub, op_mul, op_pow, op_fdiv, op_idiv, op_mod,
			op_or, op_xor, op_and, op_lsh, op_rsh,
			op_lt, op_le:
			v2 := e.pop()
			v1 := e.pop()
			if v1n, v2n, ok := assert2[Number](v1, v2); ok {
				e.push(bin_num_ops[op-op_add](v1n, v2n))
			} else if op == op_add {
				if v1s, v2s, ok := assert2[String](v1, v2); ok {
					e.push(v1s + v2s)
					break
				}
				e.pushf(
					"attempt to %s %s and %s",
					op_names[op], v1.TypeOf(), v2.TypeOf(),
				)
				goto unwind
			}
		case op_goto:
			fr.pc += int(int16(fr.read_short()))
		case op_goto_if_false:
			jump := int(int16(fr.read_short()))
			if !to_boolean(e.peek1()) {
				fr.pc += jump
			}
		case op_goto_if_nihil:
			jump := int(int16(fr.read_short()))
			if is[Nihil](e.peek1()) {
				fr.pc += jump
			}
		case op_call:
			argc := fr.read_var()
			callee := e.peek(-1 - argc)
			if err := e.call_value(callee, argc); err != nil {
				e.push(err)
				goto unwind
			}
			fr = e.current_frame()
		case op_call_spread:
			argc := fr.read_var()
			if spr, ok := e.pop().(*Structure); ok {
				var i Number
				for i = 0; ; i++ {
					v := spr.Load(i)
					if !is[Nihil](v) {
						e.push(v)
						argc++
					} else {
						break
					}
				}
			} else {
				e.pushf("attempt to spread %s", spr.TypeOf())
				goto unwind
			}
			callee := e.peek(-1 - argc)
			if err := e.call_value(callee, argc); err != nil {
				e.push(err)
				goto unwind
			}
			fr = e.current_frame()
		case op_return:
			r := e.pop()
			e.close_upvals(fr.slots)
			e.stack = e.stack[:fr.slots-1]
			slice_pop(&e.call_stack)
			if len(e.call_stack) == 0 {
				e.status = coros_ended
				return r, nil
			}
			e.push(r)
			fr = e.current_frame()
		case op_suspend:
			e.status = coros_suspended
			return e.pop(), nil
		default:
			panic(unreachable)
		}
		continue
	unwind:
		v := e.pop()
		if !e.unwind(&fr) {
			return nil, e.uncaught(v)
		}
		r := NewStructure(Nihil{})
		r.Store(key_catched, v)
		e.push(r)
	}
}

var un_num_ops = [...]func(v Number) Value{
	0:               rev,
	op_neg - op_rev: neg,
	op_pos - op_rev: pos,
}

func rev(v Number) Value {
	return Number(int(bits.Reverse(uint(v))))
}

func neg(v Number) Value {
	return -v
}

func pos(v Number) Value {
	return Number(math.Abs(float64(v)))
}

var bin_num_ops = [...]func(v1, v2 Number) Value{
	0:                add,
	op_sub - op_add:  sub,
	op_mul - op_add:  mul,
	op_pow - op_add:  pow,
	op_fdiv - op_add: fdiv,
	op_idiv - op_add: idiv,
	op_mod - op_add:  mod,
	op_or - op_add:   or,
	op_xor - op_add:  xor,
	op_and - op_add:  and,
	op_lsh - op_add:  lsh,
	op_rsh - op_add:  rsh,
	op_lt - op_add:   lt,
	op_le - op_add:   le,
}

func add(v1, v2 Number) Value {
	return v1 + v2
}

func sub(v1, v2 Number) Value {
	return v1 - v2
}

func mul(v1, v2 Number) Value {
	return v1 * v2
}

func pow(v1, v2 Number) Value {
	return Number(math.Pow(float64(v1), float64(v2)))
}

func fdiv(v1, v2 Number) Value {
	return v1 / v2
}

func idiv(v1, v2 Number) Value {
	return Number(math.Floor(float64(v1 / v2)))
}

func mod(v1, v2 Number) Value {
	return Number(math.Mod(float64(v1), float64(v2)))
}

func or(v1, v2 Number) Value {
	return Number(int(v1) | int(v2))
}

func xor(v1, v2 Number) Value {
	return Number(int(v1) ^ int(v2))
}

func and(v1, v2 Number) Value {
	return Number(int(v1) & int(v2))
}

func lsh(v1, v2 Number) Value {
	return Number(int(v1) << int(v2))
}

func rsh(v1, v2 Number) Value {
	return Number(int(v1) >> int(v2))
}

func lt(v1, v2 Number) Value {
	return Boolean(v1 < v2)
}

func le(v1, v2 Number) Value {
	return Boolean(v1 <= v2)
}
