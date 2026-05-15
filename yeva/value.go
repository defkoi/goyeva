package yeva

import (
	"fmt"
	"iter"
	"maps"
	"math"
	"strconv"
	"strings"
)

type Value interface {
	fmt.Stringer
	TypeOf() String
}

type err_value = Value

type Nihil empty
type Boolean bool
type Number float64
type String string

func Stringf(format string, a ...any) String {
	return String(fmt.Sprintf(format, a...))
}

/* advanced prototypes
 *
 * similar to lua's metatables
 * proto = { [".+"] = (self, other) => self.value + other.value, ... }
 * overloads:
 * -- nud: '+.', '-.', '~.', '!.'
 * -- led: '.+', '.-', '.*', './', '.**', './/', '.|', '.^', '.&', '.<<', '.>>',
 * -- -- '.<', '.<=', '.=='
 * -- undefined key: load: '.[]', store: '.[]='
 * -- call: '.()'
 * -- iterator: 'in.' | 'of.'
 * -- conversion: 'string'
 * -- garbage collector: 'delete'
 */

type StructProto interface {
	Value
	Load(key Value) Value
}

func (n Nihil) Load(key Value) Value {
	return Nihil{}
}

type Structure struct {
	data  map[Value]Value
	proto StructProto
}

func NewStructure(proto StructProto) *Structure {
	return &Structure{
		data:  make(map[Value]Value, struct_cap),
		proto: proto,
	}
}

func (s *Structure) Keys() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for k := range s.data {
			if !yield(k) {
				return
			}
		}
	}
}

func (s *Structure) Values() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for _, v := range s.data {
			if !yield(v) {
				return
			}
		}
	}
}

func (s *Structure) Pairs() iter.Seq2[Value, Value] {
	return func(yield func(Value, Value) bool) {
		for k, v := range s.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (s *Structure) Size() int { return len(s.data) }

func (s *Structure) Merge(v *Structure) {
	maps.Copy(s.data, v.data)
}

func (s *Structure) Store(k Value, v Value) err_value {
	if is[Nihil](k) {
		return Stringf("structure key is %s", nihil_literal)
	}
	if is[Nihil](v) {
		delete(s.data, k)
	} else {
		s.data[k] = v
	}
	return nil
}

func (s *Structure) Load(k Value) (v Value) {
	if v, ok := s.data[k]; ok {
		return v
	} else {
		return s.proto.Load(k)
	}
}

type upval_info struct {
	location int
	is_local bool
	upval2   upval2_info
}

type upval2_info any

type upval2_open_info struct {
	loc  int
	back int /* if raw index */
}

type upval2_name_info up2_name

type definition struct {
	// prototype
	name   string
	paramc int
	vararg bool

	// chunk
	code   []uint8
	lines  []int
	values []Value
	defs   []*definition

	// closure
	upvals []upval_info
}

func (d *definition) write_code(op op_code, line int) {
	slice_push(&d.code, op)
	slice_push(&d.lines, line)
}

func (d *definition) add_value(v Value) int {
	for i, sv := range d.values {
		if sv == v {
			return i
		}
	}
	slice_push(&d.values, v)
	return len(d.values) - 1
}

func (d *definition) add_def(def *definition) int {
	slice_push(&d.defs, def)
	return len(d.defs) - 1
}

type upvalue struct {
	abs_loc int
	ref     *[]Value
	clsd    Value
	is_init bool
	up2     up2
}

type up2 any
type up2_open = int
type up2_name = String

const upvalue_is_closed = -1

func (u *upvalue) close() {
	u.clsd = (*u.ref)[u.abs_loc]
	u.abs_loc = upvalue_is_closed
	u.ref = nil
	u.up2 = nil
}

func (u *upvalue) store(e *Engine, v Value) err_value {
	if !u.is_init {
		switch up2 := u.up2.(type) {
		case up2_open:
			(*u.ref)[up2] = v
		case up2_name:
			return e.store_global(up2, v)
		default:
			panic(unreachable)
		}
	} else if u.abs_loc != upvalue_is_closed {
		(*u.ref)[u.abs_loc] = v
	} else {
		u.clsd = v
	}
	return nil
}

func (u *upvalue) load(e *Engine) (Value, err_value) {
	if !u.is_init {
		switch up2 := u.up2.(type) {
		case up2_open:
			return (*u.ref)[up2], nil
		case up2_name:
			return e.load_global(up2)
		default:
			panic(unreachable)
		}
	} else if u.abs_loc != upvalue_is_closed {
		return (*u.ref)[u.abs_loc], nil
	} else {
		return u.clsd, nil
	}
}

type Callable interface {
	Value
	call(e *Engine, argc int) err_value
}

type Closure struct {
	def    *definition
	upvals []*upvalue
}

func new_closure(def *definition, e *Engine, fr *call_frame) *Closure {
	cls := &Closure{def: def, upvals: make([]*upvalue, 0, len(def.upvals))}
	for i, upv := range def.upvals {
		if upv.is_local {
			abs_loc := fr.slots + upv.location
			slice_push(&cls.upvals, e.add_open_upval(abs_loc))
		} else {
			slice_push(&cls.upvals, fr.closure.upvals[upv.location])
		}
		e.bind_upvalue2(cls.upvals[i], upv.upval2)
	}
	return cls
}

func (c *Closure) call(e *Engine, argc int) err_value {
	if len(e.call_stack) == cap(e.call_stack) {
		panic(err_stack_overflow)
	}
	new_frame := call_frame{
		closure: c,
		pc:      0,
		slots:   len(e.stack) - argc,
	}
	slice_push(&e.call_stack, new_frame)
	e.balance_args(argc, c.def.paramc, c.def.vararg)
	return nil
}

type NativeFunc = func(*Engine, []Value) (result Value, throw err_value)

type Native struct {
	fn NativeFunc
}

func NewNative(fn NativeFunc) *Native {
	return &Native{fn: fn}
}

func (n *Native) call(e *Engine, argc int) err_value {
	res, err := n.fn(e, e.stack[len(e.stack)-argc:])
	if err != nil {
		return err
	}
	slice_cut(&e.stack, -argc-1)
	e.push(res)
	return nil
}

// coroutine status
type coros int

const (
	coros_new coros = iota
	coros_running
	coros_suspended
	coros_ended
)

type Coroutine struct {
	cls        *Closure
	call_stack []call_frame
	stack      []Value
	open_upv   *linked_node[*upvalue]
	status     coros
}

func new_coroutine(cls *Closure) *Coroutine {
	return &Coroutine{
		cls:        cls,
		call_stack: make([]call_frame, 0, call_stack_limit),
		stack:      make([]Value, 0, stack_init_cap),
	}
}

func (c *Coroutine) call(e *Engine, argc int) err_value {
	return Stringf("todo")
}

func (v Nihil) TypeOf() String      { return nihil_literal }
func (v Boolean) TypeOf() String    { return "boolean" }
func (v Number) TypeOf() String     { return "number" }
func (v String) TypeOf() String     { return "string" }
func (v *Structure) TypeOf() String { return "structure" }
func (v *Closure) TypeOf() String   { return "function" }
func (v *Native) TypeOf() String    { return "function" }
func (v *Coroutine) TypeOf() String { return "coroutine" }

func (v Nihil) String() string      { return nihil_literal }
func (v Boolean) String() string    { return strconv.FormatBool(bool(v)) }
func (v Number) String() string     { return number_format(v) }
func (v String) String() string     { return string(v) }
func (v *Structure) String() string { return fmt.Sprintf("structure: %p", v) }
func (v *Closure) String() string   { return fmt.Sprintf("function: %p", v) }
func (v *Native) String() string    { return fmt.Sprintf("function: %p", v) }
func (v *Coroutine) String() string { return fmt.Sprintf("coroutine: %p", v) }

func number_format(n Number) string {
	f := float64(n)
	if math.IsNaN(f) {
		return "nan"
	} else if math.IsInf(f, 0) {
		if math.IsInf(f, 1) {
			return "inf"
		} else {
			return "-inf"
		}
	} else {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
}

func structure_format(s *Structure) string {
	var f func(s *Structure, ref map[*Structure]empty) string
	f = func(s *Structure, ref map[*Structure]empty) string {
		if len(s.data) == 0 {
			return "{}"
		}
		if ref == nil {
			ref = map[*Structure]empty{s: {}}
		} else {
			ref[s] = empty{}
		}
		var r strings.Builder
		r.WriteString("{")
		prev := false
		write := func(format string, v Value) {
			if struk, ok := v.(*Structure); ok {
				if map_has(ref, struk) {
					fmt.Fprintf(&r, format, "{...}")
				} else {
					fmt.Fprintf(&r, format, f(struk, ref))
				}
			} else if strk, ok := v.(String); ok {
				fmt.Fprintf(&r, format, fmt.Sprintf("\"%s\"", strk))
			} else {
				fmt.Fprintf(&r, format, v)
			}
		}
		for k, v := range s.data {
			if prev {
				r.WriteString(", ")
			}
			write("%v: ", k)
			write("%v", v)
			prev = true
		}
		r.WriteString("}")
		return r.String()
	}
	return f(s, nil)
}

func to_boolean(v Value) Boolean {
	switch v := v.(type) {
	case Nihil:
		return false
	case Boolean:
		return v
	case Number:
		return v != 0
	case String:
		return v != ""
	default:
		return true
	}
}
