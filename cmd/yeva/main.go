package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"goyeva/yeva"
)

func main() {
	if len(os.Args) == 1 {
		showVersion()
		showUsage()
		return
	}

	var err error
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			showVersion()
		case "repl":
			err = runRepl()
		case "run":
			err = runFile(os.Args[2:])
		default:
			err = fmt.Errorf("unknown command '%s'", os.Args[1])
		}
	}

	if err != nil {
		log.Fatal(err)
	}
}

func runFile(args []string) error {
	if len(args) == 0 {
		return errors.New("run file: no file")
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("run file: %w", err)
	}
	if _, err = yeva.New().Interpret(src); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return nil
}

func runRepl() error {
	vm := yeva.New()
	showVersion()
	fmt.Println("exit using ctrl+c")
	for {
		fmt.Print("> ")
		src, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Print("\b\b")
				return nil
			}
			return fmt.Errorf("run repl: %w", err)
		}
		if _, err = vm.Interpret(src); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func showUsage() {
	fmt.Println("usage:")
	fmt.Println(format(yeva.Name+" [command] [...arguments]", ""))
	fmt.Println()
	fmt.Println("commands:")
	fmt.Println(format("repl", "run eval print loop"))
	fmt.Println(format("run", "execute script"))
	fmt.Println(format("version", "show version"))
}

func showVersion() { fmt.Printf("%s v%v\n", yeva.Name, yeva.Version) }

func format(arg, desc string) string {
	return fmt.Sprintf("    %-18s%s", arg, strings.ToLower(desc))
}
