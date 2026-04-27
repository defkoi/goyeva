package yeva

import (
	"fmt"
)

type lexer struct {
	source   []byte
	cursor   int
	start    int
	line     int
	new_line bool
}

func new_lexer(src []byte) lexer {
	l := lexer{
		source: src,
		line:   1,
	}
	l.skip_shebang()
	if dbg_lex {
		fmt.Println(cover("@tokens", 30, '=') + "|")
	}
	return l
}

func (l *lexer) lex() lexeme {
	line := l.line
white:
	for {
		if l.current() == '#' {
			l.skip_line()
		} else if l.current() == '/' && l.peek() == '*' {
			l.step()
			l.step()
			for l.current() != '*' && l.peek() != '/' {
				if l.current() == '\n' {
					l.line++
				}
				if l.current() == nul {
					return l.error_lexeme("unfinished block comment")
				}
				l.step()
			}
			l.step()
			l.step()
		}
		switch l.current() {
		case '\n':
			l.line++
			fallthrough
		case ' ', '\r', '\t':
			l.step()
		default:
			break white
		}
	}
	l.start = l.cursor
	if mode_autosemi {
		if l.new_line && line < l.line {
			return l.make_lexeme(lx_line)
		}
	}
	if l.current() == nul {
		return l.make_lexeme(lx_eof)
	}
	c := l.step()
	if trio, ok := trio[[3]byte{c, l.current(), l.peek()}]; ok {
		l.step()
		l.step()
		return l.make_lexeme(trio)
	}
	if duo, ok := duo[[2]byte{c, l.current()}]; ok {
		l.step()
		return l.make_lexeme(duo)
	}
	if solo, ok := solo[c]; ok {
		return l.make_lexeme(solo)
	}
	if is_alpha(c) {
		return l.name()
	}
	if is_numeric(c, 10) {
		return l.number()
	}
	if c == '"' {
		return l.string()
	}
	return l.error_lexeme("unexpected symbol '%c'", c)
}

func (l *lexer) skip_line() {
	for l.current() != '\n' && l.current() != nul {
		l.step()
	}
}

func (l *lexer) skip_shebang() {
	if l.current() == '#' && l.peek() == '!' {
		l.skip_line()
	}
}

func (l *lexer) literal() string {
	return string(l.source[l.start:l.cursor])
}

func (l *lexer) name_type() lx_type {
	if t, ok := keywords[l.literal()]; ok {
		return t
	}
	return lx_name
}

func (l *lexer) name() lexeme {
	for is_alnum(l.current()) {
		l.step()
	}
	return l.make_lexeme(l.name_type())
}

func (l *lexer) number() lexeme {
	var allow_underscore bool
	read := func() {
		for is_numeric(l.current(), 10) ||
			(allow_underscore && l.current() == '_') {
			allow_underscore = l.current() != '_'
			l.step()
		}
	}
	allow_underscore = true
	read()
	if !allow_underscore || is_alpha(l.current()) {
		return l.error_lexeme("malformed number")
	}
	if l.current() == '.' && is_numeric(l.peek(), 10) {
		l.step()
		allow_underscore = false
		read()
		if !allow_underscore || is_alpha(l.current()) {
			return l.error_lexeme("malformed number")
		}
	}
	return l.make_lexeme(lx_number)
}

func (l *lexer) string() lexeme {
	for cur := l.step(); cur != '"'; cur = l.step() {
		if cur == '\n' || cur == nul {
			return l.error_lexeme("unfinished string")
		} else if cur == '\\' &&
			(l.current() == '"' || l.current() == '\\') {
			l.step()
		}
	}
	return l.make_lexeme(lx_string)
}

func (l *lexer) current() byte {
	if l.cursor >= len(l.source) {
		return nul
	}
	return l.source[l.cursor]
}

func (l *lexer) peek() byte {
	if l.cursor+1 >= len(l.source) {
		return nul
	}
	return l.source[l.cursor+1]
}

func (l *lexer) step() byte {
	b := l.current()
	l.cursor++
	return b
}

func (l *lexer) make_lexeme(t lx_type) lexeme {
	l.new_line = map_has(insert_new_line_after, t)
	lx := lexeme{lx_type: t, literal: l.literal(), line: l.line}
	if dbg_lex {
		fmt.Println(lx)
	}
	return lx
}

func (l *lexer) error_lexeme(format string, a ...any) lexeme {
	return lexeme{
		lx_type: lx_error,
		literal: fmt.Sprintf(format, a...),
		line:    l.line,
	}
}

func is_alnum(b byte) bool {
	return is_alpha(b) || is_numeric(b, 10)
}

func is_alpha(b byte) bool {
	lb := lower_alpha(b)
	return 'a' <= lb && lb <= 'z' || b == '_'
}

func is_numeric(b byte, base int) bool {
	if base <= 10 {
		return '0' <= b && b <= '0'+byte(base)-1
	}
	lb := lower_alpha(b)
	return ('0' <= b && b <= '9') ||
		('a' <= lb && lb <= 'a'+byte(base)-1)
}

func lower_alpha(b byte) byte {
	return ('a' - 'A') | b
}

var insert_new_line_after = map[lx_type]empty{
	lx_rparen: {},
	lx_rbrace: {},
	lx_rbrack: {},

	lx_plus_plus:   {},
	lx_minus_minus: {},

	lx_name:   {},
	lx_number: {},
	lx_string: {},

	lx_nihil:    {},
	lx_false:    {},
	lx_true:     {},
	lx_break:    {},
	lx_continue: {},
	lx_return:   {},
}

var solo = map[byte]lx_type{
	'(': lx_lparen,
	')': lx_rparen,
	'{': lx_lbrace,
	'}': lx_rbrace,
	'[': lx_lbrack,
	']': lx_rbrack,
	'<': lx_langle,
	'>': lx_rangle,
	';': lx_semi,
	'=': lx_equal,
	'!': lx_bang,
	'.': lx_dot,
	',': lx_comma,
	'?': lx_quest,
	':': lx_colon,
	'~': lx_tilde,
	'+': lx_plus,
	'-': lx_minus,
	'*': lx_star,
	'/': lx_slash,
	'%': lx_percent,
	'|': lx_pipe,
	'^': lx_circum,
	'&': lx_amper,
}

var duo = map[[2]byte]lx_type{
	{'=', '='}: lx_equal_equal,
	{'!', '='}: lx_bang_equal,
	{'<', '='}: lx_langle_equal,
	{'>', '='}: lx_rangle_equal,
	{'+', '+'}: lx_plus_plus,
	{'-', '-'}: lx_minus_minus,
	{'*', '*'}: lx_star_star,
	{'/', '/'}: lx_slash_slash,
	{'<', '<'}: lx_langle_langle,
	{'>', '>'}: lx_rangle_rangle,
	{'-', '>'}: lx_minus_rangle,
	{'=', '>'}: lx_equal_rangle,
	{'+', '='}: lx_plus_equal,
	{'-', '='}: lx_minus_equal,
	{'*', '='}: lx_star_equal,
	{'/', '='}: lx_slash_equal,
	{'%', '='}: lx_percent_equal,
	{'|', '='}: lx_pipe_equal,
	{'^', '='}: lx_circum_equal,
	{'&', '='}: lx_amper_equal,
	{'|', '|'}: lx_pipe_pipe,
	{'&', '&'}: lx_amper_amper,
	{'?', '?'}: lx_quest_quest,
}

var trio = map[[3]byte]lx_type{
	{'.', '.', '.'}: lx_dot_dot_dot,
	{'|', '|', '='}: lx_pipe_pipe_equal,
	{'&', '&', '='}: lx_amper_amper_equal,
	{'?', '?', '='}: lx_quest_quest_equal,
	{'*', '*', '='}: lx_star_star_equal,
	{'/', '/', '='}: lx_slash_slash_equal,
	{'<', '<', '='}: lx_langle_langle_equal,
	{'>', '>', '='}: lx_rangle_rangle_equal,
}

var keywords = map[string]lx_type{
	variable_literal:  lx_variable,
	function_literal:  lx_function,
	coroutine_literal: lx_coroutine,
	nihil_literal:     lx_nihil,
	"false":           lx_false,
	"true":            lx_true,
	"if":              lx_if,
	"else":            lx_else,
	"while":           lx_while,
	"do":              lx_do,
	"for":             lx_for,
	"in":              lx_in,
	"break":           lx_break,
	"continue":        lx_continue,
	"return":          lx_return,
	"yield":           lx_yield,
	"catch":           lx_catch,
	"throw":           lx_throw,
	"typeof":          lx_typeof,
	"struct":          lx_struct,
	"destruct":        lx_destruct,
}
