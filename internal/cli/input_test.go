package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInlineSources(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"positional", []string{`{"x":1}`, "--compact"}, "{\"x\":1}\n"},
		{"explicit", []string{"--text", `{"x":1}`, "--compact"}, "{\"x\":1}\n"},
		{"equals", []string{`--text={"x":1}`, "--compact"}, "{\"x\":1}\n"},
		{"array", []string{`[1,true,null]`, "--compact"}, "[1,true,null]\n"},
		{"negative", []string{"-12.300e+9"}, "-12.300e+9\n"},
		{"string", []string{`"a\nb"`, "--unescape"}, "a\nb\n"},
		{"extract-after", []string{`{"message":"a\nb"}`, "--extract", "message"}, "a\nb\n"},
		{"mixed-order", []string{"--indent", "4", `{"x":1}`, "--compact"}, "{\"x\":1}\n"},
		{"null", []string{"null"}, "null\n"},
		{"bool", []string{"false"}, "false\n"},
		{"documents", []string{"true false", "--compact"}, "true\nfalse\n"},
		{"stdin-explicit", []string{"--file", "-", "--compact"}, "{\"stdin\":true}\n"},
		{"stdin-operand", []string{"-", "--compact"}, "{\"stdin\":true}\n"},
		{"inline-stream", []string{"--stream", "--compact", "{\"x\":1}\n{\"x\":2}"}, "{\"x\":1}\n{\"x\":2}\n"},
		{"loose", []string{`{message:'a\nb',}`, "--loose", "--extract", "message"}, "a\nb\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, err := invoke(tc.args, `{"stdin":true}`)
			if code != 0 || out != tc.want {
				t.Fatalf("%d %q %s", code, out, err)
			}
		})
	}
}

func TestFileSources(t *testing.T) {
	for _, name := range []string{"log.json", "2026.json", "input with spaces.json", "-input.json"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(`{"id":9007199254740993}`), 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{path, "--compact"}, {"--file", path, "--compact"}, {"--compact", "--", path}} {
			code, out, err := invoke(args, "invalid stdin")
			if code != 0 || out != "{\"id\":9007199254740993}\n" {
				t.Fatalf("%v: %d %q %s", args, code, out, err)
			}
		}
	}
}

func TestInputErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--text", "{}", "--file", "log.json"}, {"--text", "{}", "{}"},
		{"--file", "log.json", "{}"}, {"{}", "[]"}, {"--text", ""}, {""},
		{"{bad}"}, {"--text", "not JSON"}, {"--text"}, {"{}", "--unknown"},
		{"{}", "--extract"}, {"{}", "--max-bytes", "1"},
	} {
		code, out, err := invoke(args, `{"stdin":true}`)
		if code == 0 || out != "" || err == "" {
			t.Fatalf("%v: %d %q %s", args, code, out, err)
		}
	}
	code, _, err := invoke([]string{"--", "--unknown"}, "")
	if code != 1 || !strings.Contains(err, "open input file") {
		t.Fatalf("terminator: %d %s", code, err)
	}
	code, _, err = invoke([]string{"--help"}, "")
	if code != 0 || !strings.Contains(err, "--text") {
		t.Fatalf("help: %d %s", code, err)
	}
}
