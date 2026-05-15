package yeva

import "fmt"

type lx_type string

const (
	lx_lparen lx_type = "("
	lx_rparen lx_type = ")"
	lx_lbrace lx_type = "{"
	lx_rbrace lx_type = "}"
	lx_lbrack lx_type = "["
	lx_rbrack lx_type = "]"

	/* lexical scopes
	 *
	 * { -> op_begin_scope
	 * variable a -> op_define_name
	 * a = 1 -> op_store_name
	 * a -> op_load_name
	 * } -> op_end_scope
	 *
	 * scope = map[name](value|value_cell)
	 * value_cell = {is_mut?: boolean, is_init?: boolean, value: value}
	 * mutable -> let | const
	 * init -> temporal dead zone
	 */

	lx_langle       lx_type = "<"
	lx_rangle       lx_type = ">"
	lx_langle_equal lx_type = "<="
	lx_rangle_equal lx_type = ">="

	lx_langle_langle       lx_type = "<<"
	lx_rangle_rangle       lx_type = ">>"
	lx_langle_langle_equal lx_type = "<<="
	lx_rangle_rangle_equal lx_type = ">>="

	lx_semi  lx_type = ";"
	lx_equal lx_type = "="
	lx_bang  lx_type = "!" // "not"?
	lx_dot   lx_type = "."
	lx_comma lx_type = ","
	lx_quest lx_type = "?" // "then"?
	lx_colon lx_type = ":" // "else"?
	lx_tilde lx_type = "~"

	lx_plus_plus   lx_type = "++"
	lx_minus_minus lx_type = "--"

	lx_minus_rangle lx_type = "->" // what about "::"?
	lx_equal_rangle lx_type = "=>"

	/* inline declarations and multiple assignments
	 *
	 * use ':=' for new variables
	 * a := 1; b := 2;
	 * b = (a := 1) + 1;
	 * 'while' and 'do' statements always create an invisible block
	 * like 'for' and 'if'
	 *
	 * change ',' behavior to support multiple assignment
	 * a, b, z = (c, d := get());
	 * z == void, a == c, b == d
	 * but if 'a, b, z = (c, d := get()) + 1;' then a == c + 1, b == z == void
	 * rules are very similar to lua,
	 * but a muiltiple assignment may be inside an expression
	 *
	 * { .a, .b } := { .a = 1, .b = 2 };
	 */

	lx_plus        lx_type = "+"
	lx_minus       lx_type = "-"
	lx_star        lx_type = "*"
	lx_star_star   lx_type = "**"
	lx_slash       lx_type = "/"
	lx_slash_slash lx_type = "//"
	lx_percent     lx_type = "%"
	lx_pipe        lx_type = "|"
	lx_circum      lx_type = "^"
	lx_amper       lx_type = "&"

	lx_plus_equal    lx_type = "+="
	lx_minus_equal   lx_type = "-="
	lx_star_equal    lx_type = "*="
	lx_slash_equal   lx_type = "/="
	lx_percent_equal lx_type = "%="
	lx_pipe_equal    lx_type = "|="
	lx_circum_equal  lx_type = "^="
	lx_amper_equal   lx_type = "&="

	lx_equal_equal lx_type = "=="
	lx_bang_equal  lx_type = "!="
	lx_tilde_tilde lx_type = "~~" // like [unused]
	lx_bang_tilde  lx_type = "!~" // unlike [unused]

	lx_pipe_pipe   lx_type = "||"
	lx_amper_amper lx_type = "&&"
	lx_quest_quest lx_type = "??"

	lx_pipe_pipe_equal   lx_type = "||="
	lx_amper_amper_equal lx_type = "&&="
	lx_quest_quest_equal lx_type = "??="
	lx_star_star_equal   lx_type = "**="
	lx_slash_slash_equal lx_type = "//="

	lx_dot_dot_dot lx_type = "..."

	lx_name   lx_type = "name"
	lx_number lx_type = "number"
	lx_string lx_type = "string"

	lx_variable  lx_type = "variable"
	lx_function  lx_type = "function"
	lx_coroutine lx_type = "coroutine" // what about function () * {}?

	/* async syntax
	 *
	 * function fetch(url, method) async { ... }
	 * function (url, method) async { ... };
	 * function async { ... };
	 * function () async => ...;
	 */

	/* if use 'nokeyword' structure syntax
	 * -> can use JS-like arrows and remove both declarations
	 *
	 * variable function = () => "result";
	 * variable function = (arg) => arg * 2;
	 * variable function = arg => arg * 2;
	 * variable function = arg => { return arg * 2; };
	 */

	lx_nihil    lx_type = "nihil"
	lx_false    lx_type = "false"
	lx_true     lx_type = "true"
	lx_if       lx_type = "if"
	lx_else     lx_type = "else"
	lx_while    lx_type = "while"
	lx_do       lx_type = "do"
	lx_for      lx_type = "for" // use "foreach" instead of "for (... in ...)"?
	lx_in       lx_type = "in"  // what about "of"?
	lx_break    lx_type = "break"
	lx_continue lx_type = "continue"
	lx_return   lx_type = "return"
	lx_yield    lx_type = "yield"
	lx_catch    lx_type = "catch"
	lx_throw    lx_type = "throw"
	lx_typeof   lx_type = "typeof"
	lx_struct   lx_type = "struct"
	lx_destruct lx_type = "destruct"

	/* accessors
	 *
	 * structure data = map[Value](Accessor|Property)
	 * Property = just Value
	 * Accessor = { get: () -> Value, set: (Value) -> void }
	 *
	 * variable structure = { ->a => 1, ->a=(v) => ._a = v };
	 *
	 * variable a = structure->a;
	 * variable new_a = structure->a = 2;
	 * structure.a == { .get, .set }
	 *
	 * we can:
	 * -- call_method ->() [structure property (structure)]
	 * -- get -> [structure property get()]
	 * -- set ->= [structure property set(value)]
	 *
	 * that means we need to remove method call chaining 'a->b->c();'
	 *
	 * upd: what is difference between:
	 * 1) variable property = structure->accessor;
	 * property();
	 * 2) structure->accessor();
	 * -- ??? --
	 * use '::' for method call?
	 * structure::->accessor(); ?
	 *
	 * upd: alternative
	 * Structure = map[name](Value|Accessor)
	 * Accessor = { get: () -> Value, set: (Value) -> void }
	 * if type of Structure[name] == Accessor do call Accessor.(get or set)
	 * else if == Value do default behavior
	 * it's similar to JavaScript
	 * use '<<' and '>>'?
	 * { .value <<() => self._value, .value >>(value) => self._value = value }
	 * { <<value() => self._value, >>value(value) => self._value = value }
	 * { .value() << self._value, .value(value) >> self._value = value }
	 */

	/* alternative 'nokeyword' syntax
	 *
	 * variable structure = (Prototype) { elem, [key] = value };
	 * variable structure = { elem, [key] = value };
	 *
	 * (Prototype) { elem, value = [key] } = structure;
	 * { elem, value = [key] } = structure;
	 *
	 * variable (Prototype) { elem, value = [key] } = structure;
	 * variable { elem, value = [key] } = structure;
	 *
	 * (Prototype) structure; <- set prototype nud
	 *
	 * upd: rename 'structure' to 'object' or 'table',
	 * but better use 'object' for all heap-allocated values
	 */

	/* array literal
	 *
	 * variable array = []{ 0, 1, 2, 3 };
	 * typeof array == "structure" or "array"?
	 * Array.isArray(array)?
	 * can we place something between '[]'? length?
	 * or better use '(Array){ 0, 1, 2, 3 };'?
	 */

	lx_line lx_type = "line"

	lx_error lx_type = "__error"
	lx_eof   lx_type = "__eof"
)

type lexeme struct {
	lx_type lx_type
	line    int
	literal string
}

/* lexemes
 *
 * lexeme = { type: lx_type, start: int, length: int, line: int }
 * ref to source []byte
 * compiler decides => copy or not
 * lexer can precompute some values? { .., value: Value }
 */

func (l lexeme) String() string {
	var lit string
	if l.literal != "" {
		lit = "> " + trim(l.literal, 32)
	}
	return fmt.Sprintf("%04d   | %-20s |%s", l.line, l.lx_type, lit)
}
