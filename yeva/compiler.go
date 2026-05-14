package yeva

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type compiler struct {
	*parser
	enclosing *compiler
	locals    []local
	scope     int
	loop      *loop
	def       *definition
}

func new_compiler(src []byte) compiler {
	return compiler{
		parser: new_parser(src),
		def:    &definition{name: "@script"},
	}
}

func (c *compiler) new_sub_compiler(def_name string) compiler {
	return compiler{
		parser:    c.parser,
		enclosing: c,
		def:       &definition{name: def_name},
	}
}

func (c *compiler) compile() (*definition, error) {
	for !c.step_on(lx_eof) {
		c.decl()
	}
	c.emit_nilret()
	if c.had_error {
		return nil, errors.New(c.error_string.String())
	}
	return c.def, nil
}

func (c *compiler) decl() {
	switch {
	case c.check_next(lx_name) &&
		c.step_on(lx_function):
		c.function_decl()
	case c.check_next(lx_name) &&
		c.step_on(lx_coroutine):
		c.coroutine_decl()
	case c.check_next(lx_name) &&
		c.step_on(lx_struct):
		c.structure_decl()
	case c.step_on(lx_variable):
		c.multiline_decl(c.variable_decl)
	default:
		c.stmt()
	}
	if c.panic_mode {
		c.sync()
	}
}

func (c *compiler) multiline_decl(f func()) {
	many := c.step_on(lx_lparen)
	for {
		if many && c.step_on(lx_rparen) {
			c.expect_semi()
			break
		}
		f()
		if !many {
			break
		}
	}
}

func (c *compiler) variable_decl() {
	for _, name := range c.variables() {
		c.declare_variable(name)
	}
	c.define_variables()
	c.expect_semi()
}

func (c *compiler) variables() (names []string) {
	for {
		if c.step_on(lx_destruct) {
			c.expect(lx_lbrace)
			l_state := c.save_state()
			c.skip(lx_lbrace, lx_rbrace)
			c.expect(lx_equal)
			c.expr(false)
			r_state := c.save_state()
			c.load_state(l_state)
			names = append(names, c.destruct()...)
			c.emit(op_pop)
			c.load_state(r_state)
		} else {
			names = append(names, c.expect_name())
			if c.step_on(lx_equal) {
				c.expr(false)
			} else {
				c.emit(op_nihil)
			}
		}
		if !c.step_on(lx_comma) {
			break
		}
	}
	return
}

func (c *compiler) function_decl() {
	c.declare_variable(c.expect_name())
	c.define_variables()
	if c.named_function(c.previous.literal, false) {
		c.expect_semi()
	}
}

func (c *compiler) coroutine_decl() {
	c.function_decl()
	c.emit(op_coroutine)
}

func (c *compiler) structure_decl() {
	c.declare_variable(c.expect_name())
	c.define_variables()
	c.parse_structure()
	c.expect_semi()
}

func (c *compiler) stmt() {
	switch {
	case c.step_on(lx_semi), c.step_on(lx_line):
		/* pass */
	case c.step_on(lx_lbrace):
		c.begin_scope()
		c.block()
		c.end_scope()
	case c.step_on(lx_if):
		c.if_stmt(c.step_on(lx_bang))
	case c.step_on(lx_while):
		c.while_stmt(c.step_on(lx_bang), "")
	case c.step_on(lx_do):
		c.do_stmt("")
	case c.step_on(lx_for):
		c.for_stmt("")
	case c.step_on(lx_break):
		c.break_stmt()
	case c.step_on(lx_continue):
		c.continue_stmt()
	case c.step_on(lx_return):
		c.return_stmt()
	case c.step_on(lx_throw):
		c.throw_stmt()
	case c.check_next(lx_colon) &&
		c.step_on(lx_name):
		c.label_stmt()
	default:
		c.expr(true)
		c.emit(op_pop)
		c.expect_semi()
	}
}

func (c *compiler) block() {
	for !c.check(lx_rbrace) && !c.check(lx_eof) {
		c.decl()
	}
	c.expect(lx_rbrace)
	c.ignore_line()
}

func (c *compiler) if_stmt(reverse bool) {
	c.begin_scope()
	c.expect(lx_lparen)
	if c.step_on(lx_variable) {
		c.variable_decl()
	}
	c.expr(true)
	c.expect(lx_rparen)
	if reverse {
		c.emit(op_not)
	}
	then_jump := c.emit_goto(op_goto_if_false)
	c.emit(op_pop)
	c.ignore_line()
	c.stmt()
	else_jump := c.emit_goto(op_goto)
	c.patch_goto(then_jump)
	c.emit(op_pop)
	if c.step_on(lx_else) {
		c.stmt()
	}
	c.patch_goto(else_jump)
	c.end_scope()
}

func (c *compiler) while_stmt(reverse bool, label string) {
	c.begin_loop(label)
	c.expect(lx_lparen)
	c.expr(true)
	c.expect(lx_rparen)
	if reverse {
		c.emit(op_not)
	}
	exit_jump := c.emit_goto(op_goto_if_false)
	c.emit(op_pop)
	c.ignore_line()
	c.stmt()
	c.emit_goto_back(c.loop.start)
	c.patch_goto(exit_jump)
	c.emit(op_pop)
	c.end_loop()
}

func (c *compiler) do_stmt(label string) {
	c.begin_loop(label)
	c.ignore_line()
	c.stmt()
	c.expect(lx_while)
	reverse := c.step_on(lx_bang)
	c.expect(lx_lparen)
	c.expr(true)
	if reverse {
		c.emit(op_not)
	}
	exit_jump := c.emit_goto(op_goto_if_false)
	c.emit(op_pop)
	c.emit_goto_back(c.loop.start)
	c.expect(lx_rparen)
	c.expect_semi()
	c.patch_goto(exit_jump)
	c.emit(op_pop)
	c.end_loop()
}

func (c *compiler) for_stmt(label string) {
	c.expect(lx_lparen)
	if c.find_in_parens(lx_in) {
		c.foreach_loop(label)
	} else {
		c.for_loop(label)
	}
}

func (c *compiler) for_loop(label string) {
	c.begin_scope()
	if c.step_on(lx_semi) {
		/* pass */
	} else if c.step_on(lx_variable) {
		c.variable_decl()
	} else {
		c.expr(true)
		c.emit(op_pop)
		c.expect(lx_semi)
	}
	c.begin_loop(label)
	exit_jump := -1
	if !c.step_on(lx_semi) {
		c.expr(true)
		c.expect(lx_semi)
		exit_jump = c.emit_goto(op_goto_if_false)
		c.emit(op_pop)
	}
	if !c.step_on(lx_rparen) {
		body_jump := c.emit_goto(op_goto)
		inc_start := len(c.def.code)
		c.expr(true)
		c.emit(op_pop)
		c.expect(lx_rparen)
		c.emit_goto_back(c.loop.start)
		c.loop.start = inc_start
		c.patch_goto(body_jump)
	}
	c.ignore_line()
	c.stmt()
	c.emit_goto_back(c.loop.start)
	if exit_jump != -1 {
		c.patch_goto(exit_jump)
		c.emit(op_pop)
	}
	c.end_loop()
	c.end_scope()
}

func (c *compiler) foreach_loop(label string) {
	var is_var, is_destruct bool
	var name string
	var l_state parser_state
	init := func() {
		is_var = c.step_on(lx_variable)
		is_destruct = c.step_on(lx_destruct)
		if is_destruct {
			c.expect(lx_lbrace)
			l_state = c.save_state()
			c.skip(lx_lbrace, lx_rbrace)
		} else {
			name = c.expect_name()
		}
	}
	start := func() {
		if is_var {
			c.begin_scope()
			if is_destruct {
				c.destruct_and_declare(l_state)
			} else {
				c.declare_variable(name)
			}
			c.define_variables()
		} else {
			if is_destruct {
				c.destruct_and_store(l_state)
				c.emit(op_pop)
			} else {
				store, _, arg := c.resolve_name(name)
				c.emit_encode(store, arg)
				c.emit(op_pop)
			}
		}
	}
	end := func() {
		if is_var {
			c.end_scope()
		}
	}
	c.foreach(label, init, start, end)
}

func (c *compiler) foreach(label string, init, start, end func()) {
	c.begin_scope()
	init()
	c.declare_variable("@iterator")
	c.expect(lx_in)
	c.expr(false)
	c.expect(lx_rparen)
	c.begin_loop(label)
	c.emit(op_dup, op_call, 0)
	exit_jump := c.emit_goto(op_goto_if_nihil)
	start()
	c.ignore_line()
	c.stmt()
	end()
	c.emit_goto_back(c.loop.start)
	c.patch_goto(exit_jump)
	c.emit(op_pop)
	c.end_loop()
	c.end_scope()
}

func (c *compiler) break_stmt() {
	if c.step_on_semi() {
		if c.loop != nil {
			c.end_loop_scopes(c.loop)
			slice_push(&c.loop.breaks, c.emit_goto(op_goto))
		} else {
			c.error_near_previous("'break' outside loop")
		}
	} else {
		label := c.expect_name()
		c.expect_semi()
		for loop := c.loop; loop != nil; loop = loop.enclosing {
			if loop.label == label {
				c.end_loop_scopes(loop)
				slice_push(&loop.breaks, c.emit_goto(op_goto))
				return
			}
		}
		c.error_near_previous("undefined label")
	}
}

func (c *compiler) continue_stmt() {
	if c.step_on_semi() {
		if c.loop != nil {
			c.end_loop_scopes(c.loop)
			c.emit_goto_back(c.loop.start)
		} else {
			c.error_near_previous("'continue' outside loop")
		}
	} else {
		label := c.expect_name()
		c.expect_semi()
		for loop := c.loop; loop != nil; loop = loop.enclosing {
			if loop.label == label {
				c.end_loop_scopes(loop)
				c.emit_goto_back(loop.start)
				return
			}
		}
		c.error_near_previous("undefined label")
	}
}

func (c *compiler) return_stmt() {
	if c.step_on_semi() {
		c.emit_nilret()
	} else {
		c.expr(true)
		c.expect_semi()
		c.emit(op_return)
	}
}

func (c *compiler) throw_stmt() {
	c.expr(true)
	c.emit(op_throw)
	c.expect_semi()
}

func (c *compiler) label_stmt() {
	name := c.previous.literal
	c.step()
	switch {
	case c.step_on(lx_while):
		c.while_stmt(false, name)
	case c.step_on(lx_do):
		c.do_stmt(name)
	case c.step_on(lx_for):
		c.for_stmt(name)
	default:
		c.stmt()
	}
}

func (c *compiler) expr(allow_comma bool) {
	if allow_comma {
		c.precedence(prec_comma, 0)
	} else {
		c.precedence(prec_assign, 0)
	}
}

func (c *compiler) precedence(prec precedence, prefix int) {
	nud_fn := c.nud()
	c.step()
	if nud_fn == nil {
		c.error_near_previous("expression expected")
		return
	}
	can_assign := prec <= prec_assign
	if c.if_assign(nud_fn(), can_assign, prefix) {
		return
	}
	for prec <= precedences[c.current.lx_type] {
		led_fn := c.led()
		c.step()
		if c.if_assign(led_fn(), can_assign, prefix) {
			return
		}
	}
	if prefix != 0 {
		c.error_near_previous("invalid prefix")
	} else if c.current.lx_type == lx_minus_minus ||
		c.current.lx_type == lx_plus_plus {
		c.error_near_previous("invalid postfix")
	} else if can_assign && map_has(assign_lexemes, c.current.lx_type) {
		c.error_near_previous("invalid assignment")
	}
}

var assign_lexemes = map[lx_type]empty{
	lx_equal:             {},
	lx_plus_equal:        {},
	lx_minus_equal:       {},
	lx_star_equal:        {},
	lx_star_star_equal:   {},
	lx_slash_equal:       {},
	lx_slash_slash_equal: {},
	lx_percent_equal:     {},

	lx_pipe_equal:          {},
	lx_circum_equal:        {},
	lx_amper_equal:         {},
	lx_langle_langle_equal: {},
	lx_rangle_rangle_equal: {},

	lx_pipe_pipe_equal:   {},
	lx_amper_amper_equal: {},
	lx_quest_quest_equal: {},
}

// if_assign returns true if there was assignment through prefix.
// if buf is nil -> code already emitted, return false immediately.
func (c *compiler) if_assign(buf *emit_buf, can_assign bool, prefix int) bool {
	if buf == nil {
		return false
	}

	load := func() { c.emit(buf.load[:]...) }
	load_no_pop := func() { c.emit(buf.load_no_pop[:]...) }
	store := func() { c.emit(buf.store[:]...) }

	if prefix != 0 && precedences[c.current.lx_type] <= prec_un {
		c.assign_prefix(load_no_pop, store, prefix)
		return true
	} else if c.step_on(lx_plus_plus) {
		c.assign_postfix(load_no_pop, store, 1)
	} else if c.step_on(lx_minus_minus) {
		c.assign_postfix(load_no_pop, store, -1)
	} else if can_assign {
		switch {
		case c.step_on(lx_equal):
			c.expr(false)
			store()
		case map_has(equal_ops, c.current.lx_type):
			op := equal_ops[c.current.lx_type]
			c.step()
			load_no_pop()
			c.expr(false)
			c.emit(op)
			store()
		case c.step_on(lx_pipe_pipe_equal):
			load_no_pop()
			c.parse_or()
			store()
		case c.step_on(lx_amper_amper_equal):
			load_no_pop()
			c.parse_and()
			store()
		case c.step_on(lx_quest_quest_equal):
			load_no_pop()
			c.parse_nihillish()
			store()
		default:
			load()
		}
	} else {
		load()
	}
	return false
}

// prefix != 0
func (c *compiler) assign_prefix(load_no_pop, store func(), prefix int) {
	load_no_pop()
	c.emit_value(Number(1))
	if prefix > 0 {
		c.emit(op_add)
	} else {
		c.emit(op_sub)
	}
	store()
}

// postfix != 0
func (c *compiler) assign_postfix(load_no_pop, store func(), postfix int) {
	load_no_pop()
	c.emit(op_copy_to)
	c.emit_value(Number(1))
	if postfix > 0 {
		c.emit(op_add)
	} else {
		c.emit(op_sub)
	}
	store()
	c.emit(op_copy_from)
}

var equal_ops = map[lx_type]op_code{
	lx_plus_equal:          op_add,
	lx_minus_equal:         op_sub,
	lx_star_equal:          op_mul,
	lx_star_star_equal:     op_pow,
	lx_slash_equal:         op_fdiv,
	lx_slash_slash_equal:   op_idiv,
	lx_percent_equal:       op_mod,
	lx_pipe_equal:          op_or,
	lx_circum_equal:        op_xor,
	lx_amper_equal:         op_and,
	lx_langle_langle_equal: op_lsh,
	lx_rangle_rangle_equal: op_rsh,
}

func (c *compiler) nud() parse_func {
	switch c.current.lx_type {
	case lx_lparen:
		return c.parse_group
	case lx_name:
		return c.parse_name
	case lx_nihil, lx_false, lx_true:
		return c.parse_literal
	case lx_number:
		return c.parse_number
	case lx_string:
		return c.parse_string
	case lx_struct:
		return c.parse_structure
	case lx_destruct:
		return c.parse_destruct
	case lx_function:
		return c.parse_function
	case lx_coroutine:
		return c.parse_coroutine
	case lx_plus, lx_minus, lx_bang, lx_typeof, lx_tilde,
		lx_plus_plus, lx_minus_minus:
		return c.parse_prefix
	case lx_catch:
		return c.parse_catch
	default:
		return nil
	}
}

func (c *compiler) parse_group() *emit_buf {
	c.expr(true)
	c.expect(lx_rparen)
	return nil
}

func (c *compiler) parse_name() *emit_buf {
	store, load, arg := c.resolve_name(c.previous.literal)
	return new_emit_buf_encode(load, load, store, arg)
}

func (c *compiler) resolve_name(name string) (store, load op_code, arg int) {
	if idx, ok := c.resolve_local(name); ok {
		arg = idx
		store = op_store_local
		load = op_load_local
	} else if idx, ok := c.resolve_upvalue(name); ok {
		arg = idx
		store = op_store_upvalue
		load = op_load_upvalue
	} else {
		arg = c.def.add_value(String(name))
		store = op_store_name
		load = op_load_name
	}
	return
}

func (c *compiler) resolve_local(name string) (int, bool) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].name == name && c.locals[i].is_init {
			return i, true
		}
	}
	return 0, false
}

func (c *compiler) resolve_upvalue(name string) (int, bool) {
	if c.enclosing == nil {
		return 0, false
	} else if l, ok := c.enclosing.resolve_local(name); ok {
		c.enclosing.locals[l].is_upval = true
		return c.add_upvalue(l, true), true
	} else if u, ok := c.enclosing.resolve_upvalue(name); ok {
		return c.add_upvalue(u, false), true
	}
	return 0, false
}

func (c *compiler) add_upvalue(idx int, is_local bool) int {
	new_upv := upval_info{location: idx, is_local: is_local}
	for i, upv := range c.def.upvals {
		if upv == new_upv {
			return i
		}
	}
	slice_push(&c.def.upvals, new_upv)
	return len(c.def.upvals) - 1
}

func (c *compiler) parse_literal() *emit_buf {
	switch c.previous.lx_type {
	case lx_nihil:
		c.emit(op_nihil)
	case lx_true:
		c.emit(op_true)
	case lx_false:
		c.emit(op_false)
	default:
		panic(unreachable)
	}
	return nil
}

func (c *compiler) parse_number() *emit_buf {
	n, _ := strconv.ParseFloat(c.previous.literal, 64)
	c.emit_value(Number(n))
	return nil
}

/* template literal
 *
 * if "%name" or "%(name + "!"->repeat(3))" =>
 * create local template lexer =>
 * parse one name or until ')'
 *
 * this means string literals can contain newlines
 */

func (c *compiler) parse_string() *emit_buf {
	s := []rune(c.previous.literal[1 : len(c.previous.literal)-1])
	r := strings.Builder{}
	for i := 0; i < len(s); i++ {
		cur := s[i]
		if cur == '\\' {
			var next rune
			if i == len(s)-1 {
				next = nul
			} else {
				next = s[i+1]
			}
			if esc, ok := escapes[next]; ok {
				r.WriteByte(esc)
				i++
			} else {
				c.error_near_previous("invalid escape")
			}
		} else {
			r.WriteRune(cur)
		}
	}
	c.emit_value(String(r.String()))
	return nil
}

var escapes = map[rune]byte{
	'a':  '\a',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'v':  '\v',
	'"':  '"',
	'\\': '\\',
}

func (c *compiler) parse_structure() *emit_buf {
	if c.step_on(lx_lparen) {
		c.expr(false)
		c.expect(lx_rparen)
	} else {
		c.emit(op_nihil)
	}
	c.emit(op_structure)
	c.expect(lx_lbrace)
	c.struct_body()
	return nil
}

func (c *compiler) struct_body() {
	var i float64 = 0
	if !c.check(lx_rbrace) {
		for {
			if c.step_on(lx_dot) {
				name := c.expect_name()
				c.emit_value(String(name))
				if c.step_on(lx_equal) {
					c.expr(false)
				} else if c.check(lx_lparen) ||
					c.check(lx_lbrace) ||
					c.check(lx_equal_rangle) {
					c.named_function(name, false)
				} else {
					c.emit(c.parse_name().load...)
				}
				c.emit(op_define_key)
			} else if c.step_on(lx_minus_rangle) {
				name := c.expect_name()
				c.emit_value(String(name))
				c.named_function(name, true)
				c.emit(op_define_key)
			} else if c.step_on(lx_lbrack) {
				c.expr(true)
				c.expect(lx_rbrack)
				c.expect(lx_equal)
				c.expr(false)
				c.emit(op_define_key)
			} else {
				c.expr(false)
				if c.step_on(lx_dot_dot_dot) {
					c.emit(op_define_key_spread)
				} else {
					c.emit_value(Number(i))
					i++
					c.emit(op_swap, op_define_key)
				}
			}
			c.ignore_line()
			if !c.step_on(lx_comma) {
				break
			}
			if c.check(lx_rbrace) {
				break
			}
		}
	}
	c.expect(lx_rbrace)
}

func (c *compiler) destruct() (vnames []string) {
	array_index := 0
	for {
		c.emit(op_dup)
		if c.step_on(lx_dot) {
			name := c.expect_name()
			vnames = append(vnames, name)
			c.emit_value(String(name))
		} else {
			vnames = append(vnames, c.expect_name())
			if c.step_on(lx_equal) {
				if c.step_on(lx_lbrack) {
					c.expr(true)
					c.expect(lx_rbrack)
				} else {
					c.expect(lx_dot)
					c.emit_value(String(c.expect_name()))
				}
			} else {
				c.emit_value(Number(array_index))
				array_index++
			}
		}
		c.emit(op_load_key)
		if c.step_on(default_lexeme) {
			c.nihillish(prec_assign)
		}
		c.emit(op_swap)
		if !c.step_on(lx_comma) {
			break
		}
		if c.check(lx_rbrace) {
			break
		}
	}
	c.expect(lx_rbrace)
	return vnames
}

func (c *compiler) parse_function() *emit_buf {
	name := "@anonymous"
	if last := slice_last(c.locals); last != nil && !last.is_init {
		name = last.name
	}
	c.named_function(name, false)
	return nil
}

func (c *compiler) parse_coroutine() *emit_buf {
	c.parse_function()
	c.emit(op_coroutine)
	return nil
}

func (c *compiler) named_function(name string, is_method bool) bool {
	fc := c.new_sub_compiler(name)
	if is_method {
		fc.def.paramc++
		fc.declare_variable(name_self)
		slice_last(fc.locals).is_init = true
	}
	if fc.step_on(lx_lparen) {
		fc.param_list()
	}
	is_arrow := fc.step_on(lx_equal_rangle)
	if is_arrow {
		fc.expr(false)
		fc.emit(op_return)
	} else {
		fc.expect(lx_lbrace)
		fc.block()
		fc.emit_nilret()
	}
	c.emit_closure(fc.def)
	return is_arrow
}

func (c *compiler) parse_prefix() *emit_buf {
	op := c.previous.lx_type
	var prefix int
	switch op {
	case lx_plus_plus:
		prefix = 1
	case lx_minus_minus:
		prefix = -1
	}
	c.precedence(prec_un, prefix)
	switch op {
	case lx_bang:
		c.emit(op_not)
	case lx_plus:
		c.emit(op_pos)
	case lx_minus:
		c.emit(op_neg)
	case lx_typeof:
		c.emit(op_typeof)
	case lx_yield:
		c.emit(op_suspend)
	case lx_plus_plus, lx_minus_minus:
		/* pass */
	default:
		panic(unreachable)
	}
	return nil
}

func (c *compiler) parse_destruct() *emit_buf {
	c.expect(lx_lbrace)
	l_state := c.save_state()
	c.skip(lx_lbrace, lx_rbrace)
	c.expect(lx_equal)
	c.expr(false)
	c.destruct_and_store(l_state)
	return nil
}

func (c *compiler) parse_catch() *emit_buf {
	catch_jump := c.emit_goto(op_begin_catch)
	c.precedence(prec_un, 0)
	c.emit(op_end_catch)
	c.patch_goto(catch_jump)
	return nil
}

func (c *compiler) led() parse_func {
	switch c.current.lx_type {
	case lx_comma:
		return c.parse_comma
	case lx_plus, lx_minus,
		lx_star, lx_slash, lx_percent,
		lx_star_star, lx_slash_slash,
		lx_equal_equal, lx_bang_equal,
		lx_langle, lx_langle_equal,
		lx_rangle, lx_rangle_equal,
		lx_pipe, lx_circum, lx_amper,
		lx_langle_langle, lx_rangle_rangle:
		return c.parse_infix
	case lx_pipe_pipe: // || or
		return c.parse_or
	case lx_amper_amper: // && and
		return c.parse_and
	case lx_quest_quest: // ??
		return c.parse_nihillish
	case lx_quest: // ? then
		return c.parse_then
	case lx_lparen: // (
		return c.parse_call
	case lx_lbrack: // [
		return c.parse_index
	case lx_dot: // .
		return c.parse_dot
	case lx_minus_rangle: // ->
		return c.parse_arrow
	default:
		panic(unreachable)
	}
}

func (c *compiler) parse_comma() *emit_buf {
	c.emit(op_pop)
	c.expr(true)
	return nil
}

func (c *compiler) parse_infix() *emit_buf {
	lxt := c.previous.lx_type
	c.precedence(
		precedences[lxt]+
			precedence(btoi(!map_has(right_asso, lxt))), 0,
	)
	switch lxt {
	case lx_bang_equal:
		c.emit(op_eq, op_not)
	case lx_equal_equal:
		c.emit(op_eq)
	case lx_langle:
		c.emit(op_lt)
	case lx_langle_equal:
		c.emit(op_le)
	case lx_rangle:
		c.emit(op_le, op_not)
	case lx_rangle_equal:
		c.emit(op_lt, op_not)
	case lx_plus:
		c.emit(op_add)
	case lx_minus:
		c.emit(op_sub)
	case lx_star:
		c.emit(op_mul)
	case lx_star_star:
		c.emit(op_pow)
	case lx_slash:
		c.emit(op_fdiv)
	case lx_slash_slash:
		c.emit(op_idiv)
	case lx_percent:
		c.emit(op_mod)
	case lx_pipe:
		c.emit(op_or)
	case lx_circum:
		c.emit(op_xor)
	case lx_amper:
		c.emit(op_and)
	case lx_langle_langle:
		c.emit(op_lsh)
	case lx_rangle_rangle:
		c.emit(op_rsh)
	default:
		panic(unreachable)
	}
	return nil
}

var right_asso = map[lx_type]empty{
	lx_star_star: {},
}

func (c *compiler) parse_or() *emit_buf {
	left_jump := c.emit_goto(op_goto_if_false)
	right_jump := c.emit_goto(op_goto)
	c.patch_goto(left_jump)
	c.emit(op_pop)
	c.precedence(prec_lor, 0)
	c.patch_goto(right_jump)
	return nil
}

func (c *compiler) parse_and() *emit_buf {
	end_jump := c.emit_goto(op_goto_if_false)
	c.emit(op_pop)
	c.precedence(prec_land, 0)
	c.patch_goto(end_jump)
	return nil
}

func (c *compiler) parse_nihillish() *emit_buf {
	c.nihillish(prec_lor)
	return nil
}

func (c *compiler) nihillish(prec precedence) {
	left_jump := c.emit_goto(op_goto_if_nihil)
	right_jump := c.emit_goto(op_goto)
	c.patch_goto(left_jump)
	c.emit(op_pop)
	c.precedence(prec, 0)
	c.patch_goto(right_jump)
}

func (c *compiler) parse_then() *emit_buf {
	then_jump := c.emit_goto(op_goto_if_false)
	c.emit(op_pop)
	c.expr(true)
	else_jump := c.emit_goto(op_goto)
	if !c.step_on(lx_else) {
		c.expect(lx_colon)
	}
	c.patch_goto(then_jump)
	c.emit(op_pop)
	c.expr(false)
	c.patch_goto(else_jump)
	return nil
}

func (c *compiler) parse_call() *emit_buf {
	argc, is_spread := c.arg_list()
	var op = op_call
	if is_spread {
		op = op_call_spread
	}
	c.emit_encode(op, argc)
	return nil
}

func (c *compiler) parse_index() *emit_buf {
	c.expr(true)
	c.expect(lx_rbrack)
	return new_emit_buf(
		[]op_code{op_load_key},
		[]op_code{op_dup2, op_load_key},
		[]op_code{op_store_key},
	)
}

func (c *compiler) parse_dot() *emit_buf {
	c.expect(lx_name)
	c.emit_value(String(c.previous.literal))
	return new_emit_buf(
		[]op_code{op_load_key},
		[]op_code{op_dup2, op_load_key},
		[]op_code{op_store_key},
	)
}

func (c *compiler) parse_arrow() *emit_buf {
	c.emit(op_dup)
	for {
		c.emit_value(String(c.expect_name()))
		c.emit(op_load_key)
		if !c.step_on(lx_minus_rangle) {
			c.emit(op_swap)
			break
		}
	}
	c.expect(lx_lparen)
	argc, is_spread := c.arg_list()
	if is_spread {
		c.emit_encode(op_call_spread, argc+1)
	} else {
		c.emit_encode(op_call, argc+1)
	}
	return nil
}

func (c *compiler) declare_variable(name string) {
	if name == "_" {
		name = ""
	} else {
		for i := len(c.locals) - 1; i >= 0; i-- {
			if c.locals[i].scope < c.scope {
				break
			}
			if c.locals[i].name == name {
				c.error_near_previous("variable already declared")
			}
		}
	}
	c.add_local(name)
}

func (c *compiler) define_variables() {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if local := &c.locals[i]; !local.is_init {
			local.is_init = true
		} else {
			break
		}
	}
}

func (c *compiler) add_local(name string) {
	slice_push(&c.locals, local{
		name:  name,
		scope: c.scope,
	})
}

func (c *compiler) emit(ops ...op_code) {
	for _, op := range ops {
		c.def.write_code(op, c.previous.line)
	}
}

func (c *compiler) emit_encode(op op_code, to_enc int) {
	c.emit(op)
	c.emit(encode(to_enc)...)
}

func (c *compiler) emit_value(v Value) {
	c.emit_encode(op_value, c.def.add_value(v))
}

func (c *compiler) emit_closure(f *definition) {
	c.emit_encode(op_closure, c.def.add_def(f))
}

func (c *compiler) emit_nilret() {
	c.emit(op_nihil, op_return)
}

func (c *compiler) emit_goto(op op_code) int {
	c.emit(op, 0xff, 0xff)
	return len(c.def.code)
}

func (c *compiler) patch_goto(start int) {
	jump := len(c.def.code) - start
	if jump > math.MaxInt16 {
		c.error_near_previous("too long jump")
	}
	c.def.code[start-2], c.def.code[start-1] = u16tou8(uint16(jump))
}

func (c *compiler) emit_goto_back(start int) {
	c.emit(op_goto)
	jump := start - len(c.def.code) - 2
	if jump < math.MinInt16 {
		c.error_near_previous("too long jump")
	}
	c.emit(u16tou8(uint16(jump)))
}

func (c *compiler) begin_scope() { c.scope++ }

func (c *compiler) end_scope() {
	c.scope--
	c.close_locals(c.scope, true)
}

func (c *compiler) end_loop_scopes(l *loop) {
	c.close_locals(l.scope, false)
}

func (c *compiler) close_locals(until int, cut bool) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].scope > until {
			if c.locals[i].is_upval {
				c.emit(op_close_upvalue)
			} else {
				c.emit(op_pop)
			}
			if cut {
				slice_pop(&c.locals)
			}
		} else {
			break
		}
	}
}

func (c *compiler) begin_loop(label string) {
	c.loop = &loop{
		label:     label,
		scope:     c.scope,
		start:     len(c.def.code),
		enclosing: c.loop,
	}
}

func (c *compiler) end_loop() {
	for _, b := range c.loop.breaks {
		c.patch_goto(b)
	}
	c.loop = c.loop.enclosing
}

func (c *compiler) param_list() {
	var l_states []parser_state
	if !c.check(lx_rparen) {
		for {
			if c.step_on(lx_dot_dot_dot) {
				c.def.vararg = true
			} else {
				c.def.paramc++
			}
			if !c.def.vararg && c.step_on(lx_destruct) {
				c.expect(lx_lbrace)
				l_states = append(l_states, c.save_state())
				c.skip(lx_lbrace, lx_rbrace)
				c.declare_variable(fmt.Sprintf("@destruct%d", len(l_states)-1))
			} else {
				c.declare_variable(c.expect_name())
			}
			if !c.def.vararg && c.step_on(default_lexeme) {
				c.emit_encode(op_load_local, len(c.locals)-1)
				c.nihillish(prec_assign)
				c.emit_encode(op_store_local, len(c.locals)-1)
				c.emit(op_pop)
			}
			c.ignore_line()
			if !c.step_on(lx_comma) {
				break
			}
			if c.check(lx_rparen) || c.def.vararg {
				break
			}
		}
	}
	c.expect(lx_rparen)
	c.define_variables()
	for i, l_state := range l_states {
		_, _, arg := c.resolve_name(fmt.Sprintf("@destruct%d", i))
		c.emit_encode(op_load_local, arg)
		c.destruct_and_declare(l_state)
	}
	c.define_variables()
}

func (c *compiler) arg_list() (int, bool) {
	var argc int = 0
	is_spread := false
	if !c.check(lx_rparen) {
		for {
			c.expr(false)
			if c.step_on(lx_dot_dot_dot) {
				is_spread = true
			} else {
				argc++
			}
			c.ignore_line()
			if !c.step_on(lx_comma) {
				break
			}
			if c.check(lx_rparen) || is_spread {
				break
			}
		}
	}
	c.expect(lx_rparen)
	return argc, is_spread
}

func (c *compiler) destruct_and_store(l_state parser_state) {
	r_state := c.save_state()
	c.load_state(l_state)
	names := c.destruct()
	for i := len(names) - 1; i >= 0; i-- {
		c.emit(op_swap)
		store, _, arg := c.resolve_name(names[i])
		c.emit_encode(store, arg)
		c.emit(op_pop)
	}
	c.load_state(r_state)
}

func (c *compiler) destruct_and_declare(l_state parser_state) {
	r_state := c.save_state()
	c.load_state(l_state)
	for _, name := range c.destruct() {
		c.declare_variable(name)
	}
	c.emit(op_pop)
	c.load_state(r_state)
}

type local struct {
	name  string
	scope int

	is_init  bool
	is_upval bool
}

type loop struct {
	scope     int
	start     int
	breaks    []int
	enclosing *loop
	label     string
}

// parse_func returns *emit_buf if lvalue else nil.
type parse_func func() *emit_buf

type emit_buf struct {
	load        []op_code
	load_no_pop []op_code
	store       []op_code
}

func new_emit_buf(ops_load, ops_load_no_pop, ops_store []op_code) *emit_buf {
	return &emit_buf{
		load:        ops_load,
		load_no_pop: ops_load_no_pop,
		store:       ops_store,
	}
}

func new_emit_buf_encode(
	op_load, op_load_no_pop, op_store op_code,
	arg int,
) *emit_buf {
	b := &emit_buf{}

	b.load = append(b.load, op_load)
	b.load = append(b.load, encode(arg)...)

	b.load_no_pop = append(b.load_no_pop, op_load_no_pop)
	b.load_no_pop = append(b.load_no_pop, encode(arg)...)

	b.store = append(b.store, op_store)
	b.store = append(b.store, encode(arg)...)

	return b
}

type parser struct {
	lexer lexer

	next     lexeme
	current  lexeme
	previous lexeme

	had_error  bool
	panic_mode bool /* weird */

	error_string *strings.Builder
}

func new_parser(src []byte) *parser {
	p := &parser{lexer: new_lexer(src)}
	p.step()
	p.step()
	return p
}

type parser_state struct {
	lexer           lexer
	prev, cur, next lexeme
}

func (p *parser) save_state() parser_state {
	return parser_state{p.lexer, p.previous, p.current, p.next}
}

func (p *parser) load_state(state parser_state) {
	p.lexer, p.previous, p.current, p.next =
		state.lexer, state.prev, state.cur, state.next
}

func (p *parser) skip(l, r lx_type) {
	in := 1
	for {
		if p.check(l) {
			in++
		} else if p.check(r) {
			in--
			if in == 0 {
				p.step()
				return
			}
		} else if p.check(lx_eof) {
			return
		}
		p.step()
	}
}

func (p *parser) find_in_parens(tgt lx_type) bool {
	defer p.load_state(p.save_state())
	in := 1
	for {
		if p.check(lx_lparen) {
			in++
		} else if p.check(lx_rparen) {
			in--
			if in == 0 {
				return false
			}
		} else if p.check(tgt) {
			return true
		} else if p.check(lx_eof) {
			return false
		}
		p.step()
	}
}

func (p *parser) look_after_parens(tgt lx_type) bool {
	defer p.load_state(p.save_state())
	p.skip(lx_lparen, lx_rparen)
	return p.check(tgt)
}

func (p *parser) step() {
	p.previous = p.current
	p.current = p.next
	for {
		p.next = p.lexer.lex()
		if p.next.lx_type != lx_error {
			break
		}
		p.error_near(p.next, "%s", p.next.literal)
	}
}

func (p *parser) check(t lx_type) bool {
	return p.current.lx_type == t
}

func (p *parser) check_next(t lx_type) bool {
	return p.next.lx_type == t
}

func (p *parser) step_on(t lx_type) bool {
	if !p.check(t) {
		return false
	}
	p.step()
	return true
}

func (p *parser) step_on_semi() bool {
	if mode_autosemi {
		return p.step_on(lx_line) ||
			p.step_on(lx_semi) ||
			p.step_on(lx_eof)
	} else {
		return p.step_on(lx_semi)
	}
}

func (p *parser) expect(t lx_type) {
	if !p.step_on(t) {
		p.error_near_current("'%s' expected", t)
	}
}

func (p *parser) expect_name() string {
	p.expect(lx_name)
	return p.previous.literal
}

func (p *parser) expect_semi() {
	if mode_autosemi {
		if p.step_on(lx_line) || p.step_on(lx_eof) || p.check(lx_rbrace) {
			return
		}
	}
	p.expect(lx_semi)
}

func (p *parser) ignore_line() { p.step_on(lx_line) }

func (p *parser) error_near_previous(format string, a ...any) {
	p.error_near(p.previous, format, a...)
}

func (p *parser) error_near_current(format string, a ...any) {
	p.error_near(p.current, format, a...)
}

func (p *parser) error_near(lx lexeme, format string, a ...any) {
	if p.panic_mode {
		return
	}
	p.panic_mode = true
	p.log_error(lx, format, a...)
}

func (p *parser) log_error(lx lexeme, format string, a ...any) {
	if p.had_error {
		fmt.Fprint(p.error_string, "\n\talso: ")
	} else {
		p.error_string = &strings.Builder{}
	}
	p.had_error = true
	fmt.Fprintf(p.error_string, "ln %d: %s", lx.line, fmt.Sprintf(format, a...))
	switch lx.lx_type {
	case lx_eof, lx_line:
		fmt.Fprintf(p.error_string, " near end")
	case lx_error:
		/* pass */
	default:
		fmt.Fprintf(p.error_string, " near '%s'", lx.literal)
	}
}

func (p *parser) sync() {
	p.panic_mode = false
	for p.current.lx_type != lx_eof {
		p.step()
		if p.previous.lx_type == lx_semi ||
			p.previous.lx_type == lx_line {
			return
		}
		if _, ok := sync_lexemes[p.current.lx_type]; ok {
			return
		}
		if _, ok := sync_name_lexemes[p.current.lx_type]; ok &&
			p.next.lx_type == lx_name {
			return
		}
	}
}

var sync_lexemes = map[lx_type]empty{
	lx_variable: {},
	lx_if:       {},
	lx_while:    {},
	lx_do:       {},
	lx_for:      {},
	lx_break:    {},
	lx_continue: {},
	lx_return:   {},
	lx_throw:    {},
}

var sync_name_lexemes = map[lx_type]empty{
	lx_function:  {},
	lx_coroutine: {},
	lx_struct:    {},
}

type precedence int

const (
	prec_low precedence = iota

	prec_comma  // ,
	prec_assign // =
	prec_tern   // ? : then else
	prec_lor    // || or ??
	prec_land   // && and
	prec_or     // |
	prec_xor    // ^
	prec_and    // &
	prec_eq     // == != ~~ !~
	prec_comp   // < > <= >=
	prec_shift  // << >>
	prec_term   // + -
	prec_fact   // * / // %
	prec_pow    // **
	prec_un     // ! not + - ~ ++ -- typeof catch
	prec_call   // . ?. () ?[ [] -> :: ++ --

	prec_high
)

var precedences = map[lx_type]precedence{
	lx_comma: prec_comma, // ,

	lx_quest: prec_tern, // ? :

	lx_pipe_pipe:   prec_lor, // ||
	lx_quest_quest: prec_lor, // ??

	lx_amper_amper: prec_land, // &&

	lx_pipe:   prec_or,  // |
	lx_circum: prec_xor, // ^
	lx_amper:  prec_and, // &

	lx_equal_equal: prec_eq, // ==
	lx_bang_equal:  prec_eq, // !=

	lx_langle:       prec_comp, // <
	lx_rangle:       prec_comp, // >
	lx_langle_equal: prec_comp, // <=
	lx_rangle_equal: prec_comp, // >=

	lx_langle_langle: prec_shift, // <<
	lx_rangle_rangle: prec_shift, // >>

	lx_plus:  prec_term, // +
	lx_minus: prec_term, // -

	lx_star:        prec_fact, // *
	lx_slash:       prec_fact, // /
	lx_slash_slash: prec_fact, // //
	lx_percent:     prec_fact, // %

	lx_star_star: prec_pow, // **

	lx_lparen:       prec_call, // ()
	lx_lbrack:       prec_call, // []
	lx_dot:          prec_call, // .
	lx_minus_rangle: prec_call, // ->
}
