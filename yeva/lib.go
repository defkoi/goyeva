package yeva

import (
	"fmt"
	"slices"

	_ "embed"
)

type empty struct{}

type version struct {
	Major int
	Minor int
	Patch int
}

func (v version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

type fatal_error string

func (e fatal_error) Error() string { return string(e) }

var (
	err_stack_overflow = fatal_error("stack overflow")
)

const (
	nul = '\x00'

	mode_autosemi = true // runtime value?

	dbg_lex  = false
	dbg_code = false
	dbg_exe  = false

	// dbg_lex  = true
	// dbg_code = true
	// dbg_exe = true

	unreachable = "unreachable"

	struct_cap = 0x20

	nihil_literal     = "void"      // "nil" | "null" | "none"
	variable_literal  = "var"       // "let" | "local" | "let mut"?
	function_literal  = "function"  // what about "fn"?
	coroutine_literal = "coroutine" // what about "cr"?

	key_result  Number = 0
	key_catched Number = 1

	name_self = "self" // "this"? or use .prop + [key] instead?

	default_lexeme = lx_colon

	call_stack_limit = 0x0400

	_average_locals = 0x08
	stack_init_cap  = call_stack_limit * _average_locals
)

//go:embed embed/embed.yv
var embed []byte

func trim(s string, l int) string {
	r := []rune(s)
	if len(r) <= l {
		return s
	}
	return string(r[:l])
}

func cover(s string, w int, c rune) string {
	cvr := w - (len(s) + 2)
	if cvr < 0 {
		return s
	}
	l := mul_rune(c, cvr/2)
	r := mul_rune(c, cvr/2+cvr%2)
	return fmt.Sprintf("%s %s %s", l, s, r)
}

func mul_rune(r rune, c int) string {
	rs := make([]rune, c)
	for i := range rs {
		rs[i] = r
	}
	return string(rs)
}

func btoi(b bool) int {
	if b {
		return 1
	} else {
		return 0
	}
}

func itob(i int) bool {
	return i != 0
}

func catch[E any](on_catch func(E)) {
	if p := recover(); p != nil {
		if pe, ok := p.(E); ok {
			on_catch(pe)
		} else {
			panic(p)
		}
	}
}

func zero[T any]() (t T) { return }

func is[T Value](v Value) bool {
	_, ok := v.(T)
	return ok
}

func assert2[T Value](v1, v2 Value) (T, T, bool) {
	v1t, ok1 := v1.(T)
	v2t, ok2 := v2.(T)
	if !ok1 || !ok2 {
		return zero[T](), zero[T](), false
	}
	return v1t, v2t, true
}

func is_index(v Value) (int, bool) {
	num, ok := v.(Number)
	if !ok {
		return 0, false
	}
	idx := int(num)
	if num != Number(idx) {
		return 0, false
	}
	if idx < 0 {
		return 0, false
	}
	return idx, true
}

func map_has[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

func slice_push[T any](s *[]T, v ...T) {
	*s = append(*s, v...)
}

func slice_pop[T any](s *[]T) (t T) {
	v := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return v
}

func slice_last[T any](s []T) (t *T) {
	if len(s) == 0 {
		return
	}
	return &s[len(s)-1]
}

func slice_cut[T any](s *[]T, i int) {
	if i < 0 {
		(*s) = (*s)[:len(*s)+i]
	} else {
		(*s) = (*s)[i:len(*s)]
	}
}

const max_bool_stack8 = 0x08

type bool_stack8 struct {
	len  int8
	data int8
}

func (bs *bool_stack8) push(b bool) {
	if bs.len >= max_bool_stack8 {
		panic("bool stack overflow")
	}
	var mask int8 = 1 << bs.len
	bs.len++
	bs.data |= int8(btoi(b)) * mask
}

func (bs *bool_stack8) pop() bool {
	if bs.len == 0 {
		panic("bool stack underflow")
	}
	bs.len--
	var mask int8 = 1 << bs.len
	b := (bs.data & mask) != 0
	bs.data ^= mask
	return b
}

func (bs *bool_stack8) clear() { bs.len = 0 }

func u16tou8(u uint16) (b, s uint8) {
	return uint8((u >> 0x08) & 0xff), uint8(u & 0xff)
}

func u8tou16(b, s uint8) uint16 {
	return uint16(b)<<0x08 | uint16(s)
}

func u32tou8(u uint32) (b, mb, ms, s uint8) {
	return uint8((u >> 0x18) & 0xff),
		uint8((u >> 0x10) & 0xff),
		uint8((u >> 0x08) & 0xff),
		uint8(u & 0xff)
}

func u8tou32(b, mb, ms, s uint8) uint32 {
	return uint32(b)<<0x18 |
		uint32(mb)<<0x10 |
		uint32(ms)<<0x08 |
		uint32(s)
}

func encode(n int) []byte {
	if n == 0 {
		return []byte{0}
	}
	var bs []byte
	for n > 0 {
		bs = append(bs, byte(n&0x7f))
		n >>= 7
	}
	slices.Reverse(bs)
	for i := range len(bs) - 1 {
		bs[i] |= 1 << 7
	}
	return bs
}

func decode(b []byte) (int, int) {
	r := 0
	for i := range b {
		r |= int(b[i] & 0x7f)
		if b[i]&(1<<7) == 0 {
			return r, i + 1
		}
		r <<= 7
	}
	return r, len(b)
}

type linked_node[T any] struct {
	value T
	next  *linked_node[T]
}
