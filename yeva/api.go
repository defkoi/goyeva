package yeva

const (
	Name = "yeva"
)

var (
	Version = version{0, 0, 0}
)

func New() *engine {
	e := &engine{
		globals: map[String]Value{
			"print": &Native{native_print},
			"clock": &Native{native_clock},
			"pairs": &Native{native_pairs},
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
