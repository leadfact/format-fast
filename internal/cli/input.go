package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type inputSpec struct {
	args             []string
	text, file       string
	hasText, hasFile bool
}

// A source is selected explicitly or inferred from its syntax. JSON-looking
// operands always mean text, independently of files present in the directory.
// --file and --text make otherwise ambiguous input deterministic.
func openInput(stdin io.Reader, spec inputSpec) (io.Reader, io.Closer, error) {
	if len(spec.args) > 1 {
		return nil, nil, errors.New("expected one JSON text or file; wrap inline JSON in shell single quotes")
	}
	if spec.hasText && spec.hasFile || len(spec.args) > 0 && (spec.hasText || spec.hasFile) {
		return nil, nil, errors.New("choose one input source: positional input, --text, or --file")
	}
	if spec.hasText {
		return strings.NewReader(spec.text), nil, nil
	}
	path := spec.file
	if !spec.hasFile {
		if len(spec.args) == 0 || spec.args[0] == "-" {
			return stdin, nil, nil
		}
		path = spec.args[0]
		if looksLikeJSON(path) {
			return strings.NewReader(path), nil, nil
		}
	}
	if path == "-" {
		return stdin, nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open input file: %w (use --text for inline JSON)", err)
	}
	return file, file, nil
}

func looksLikeJSON(input string) bool {
	s := strings.TrimLeft(input, " \t\r\n\ufeff")
	if s == "" {
		return true
	} // Explicit empty text gets a parser error, not stdin.
	switch s[0] {
	case '{', '[', '"', '\'':
		return true
	}
	if strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/*") {
		return true
	}
	for _, literal := range []string{"true", "false", "null"} {
		if s == literal || strings.HasPrefix(s, literal) && len(s) > len(literal) && strings.ContainsRune(" \r\n\t", rune(s[len(literal)])) {
			return true
		}
	}
	// Only numeric syntax characters: a path such as 2026.json stays a path.
	if s[0] == '-' || s[0] >= '0' && s[0] <= '9' {
		return strings.Trim(s, "0123456789eE+-. \r\n\t") == ""
	}
	return false
}

// The standard flag package stops at its first operand. Reorder only recognized
// flags and their values, preserving operand order and the -- terminator.
// Negative JSON numbers are operands, not short options.
func orderArgs(set *flag.FlagSet, args []string) ([]string, error) {
	flags := make([]string, 0, len(args))
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") || looksLikeJSON(arg) {
			operands = append(operands, arg)
			continue
		}
		name, _, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		option := set.Lookup(name)
		if option == nil {
			if name == "h" || name == "help" {
				return []string{arg}, nil
			}
			return nil, fmt.Errorf("unknown flag %q", arg)
		}
		flags = append(flags, arg)
		boolean, ok := option.Value.(interface{ IsBoolFlag() bool })
		if hasValue || ok && boolean.IsBoolFlag() {
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag %s requires a value", arg)
		}
		i++
		flags = append(flags, args[i])
	}
	return append(append(flags, "--"), operands...), nil
}
