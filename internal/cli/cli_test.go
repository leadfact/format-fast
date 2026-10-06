package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func invoke(args []string, src string) (int, string, string) {
	var out, err bytes.Buffer
	code := Run(args, strings.NewReader(src), &out, &err)
	return code, out.String(), err.String()
}
func TestCLI(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		src, want string
	}{
		{"format", nil, `{"x":1}`, "{\n  \"x\": 1\n}\n"},
		{"extract", []string{"-extract", "message"}, `{"message":"a\nb\n"}`, "a\nb\n"},
		{"quoted", []string{"-extract", "message", "-json"}, `{"message":"a\nb"}`, "\"a\\nb\"\n"},
		{"unescape", []string{"-unescape"}, `"a\\nb"`, "a\\nb\n"},
		{"stream", []string{"-stream", "-extract", "message"}, "{\"message\":\"a\"}\n\n{\"message\":\"b\"}", "a\nb\n"},
		{"root", []string{"-extract", "", "-compact"}, `{"x":1}`, "{\"x\":1}\n"},
		{"array", []string{"-extract", "/items/0"}, `{"items":[{"a":1}]}`, "{\n  \"a\": 1\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, err := invoke(tc.args, tc.src)
			if code != 0 || out != tc.want {
				t.Fatalf("code=%d out=%q err=%s", code, out, err)
			}
		})
	}
}

func TestErrorsAndAtomicOutput(t *testing.T) {
	for _, args := range [][]string{{}, {"-extract", "missing"}, {"-max-bytes", "2"}, {"-unescape"}, {"-indent", "9"}, {"-color", "bad"}, {"-indent", "0"}, {"-max-depth", "0"}} {
		code, out, _ := invoke(args, `{"x":1,}`)
		if code == 0 || out != "" {
			t.Fatalf("partial output %v: %d %q", args, code, out)
		}
	}
	path := filepath.Join(t.TempDir(), "result.json")
	os.WriteFile(path, []byte("original"), 0600)
	code, _, _ := invoke([]string{"-output", path}, `{"x":}`)
	if code == 0 {
		t.Fatal("invalid accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "original" {
		t.Fatal("output clobbered")
	}
	code, _, err := invoke([]string{"-output", path, "-compact"}, `{"x":1}`)
	if code != 0 {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != "{\"x\":1}\n" {
		t.Fatalf("output %q", got)
	}
}

func TestStreamLimitsAndLongLine(t *testing.T) {
	src := `{"message":"` + strings.Repeat("a", 100000) + `"}` + "\n"
	code, out, err := invoke([]string{"-stream", "-extract", "message"}, src)
	if code != 0 || len(out) != 100001 {
		t.Fatalf("%d len=%d %s", code, len(out), err)
	}
	code, _, err = invoke([]string{"-stream", "-max-bytes", "20"}, src)
	if code == 0 || !strings.Contains(err, "line 1 exceeds") {
		t.Fatalf("limit: %s", err)
	}
	code, out, err = invoke([]string{"-stream", "-compact"}, "{\"x\":1}\n{bad}\n")
	if code == 0 || out != "{\"x\":1}\n" || !strings.Contains(err, "line 2") {
		t.Fatalf("%d %q %s", code, out, err)
	}
}

func TestColor(t *testing.T) {
	code, out, err := invoke([]string{"-color", "always"}, `{"x":1,"s":"\u001b[31m"}`)
	if code != 0 || !strings.Contains(out, "\x1b[36m") {
		t.Fatalf("%q %s", out, err)
	}
	code, out, _ = invoke([]string{"-color", "never"}, `{"x":1}`)
	if code != 0 || strings.Contains(out, "\x1b") {
		t.Fatal("color in plain output")
	}
}
