package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPreservesTokens(t *testing.T) {
	src := []byte(`{"amount":1.2300,"id":9007199254740993,"x":1,"x":2,"negative":-0,"exponent":1E+09,"message":"a\n\tb","empty":{},"items":[true,null,"😺"]}`)
	got, err := Format(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := json.Indent(&want, src, "", "  "); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("got:\n%s\nwant:\n%s", got, &want)
	}
	compact, err := Format(got, Options{Compact: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, compact) {
		t.Fatalf("token changed: %s", compact)
	}
}

func TestExtract(t *testing.T) {
	src := []byte(`{"message":"\nHEADERS:\n\tAccept: application/json\n","message":"second","events":[{"a/b":{"~key":"\uD83D\uDE3A"}}],"log.level":"debug"}`)
	values, err := Extract(src, "message", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("got %d matches", len(values))
	}
	got, err := DecodeString(values[0].Raw, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "\nHEADERS:\n\tAccept: application/json\n" {
		t.Fatalf("decoded %q", got)
	}
	values, err = Extract(src, "/events/0/a~1b/~0key", Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err = DecodeString(values[0].Raw, false)
	if err != nil || got != "😺" {
		t.Fatalf("%q %v", got, err)
	}
	values, err = Extract(src, "log.level", Options{})
	if err != nil || len(values) != 1 {
		t.Fatalf("literal key: %v %v", values, err)
	}
	if _, err = Extract(src, "/bad~2key", Options{}); err == nil {
		t.Fatal("invalid pointer accepted")
	}
	values, err = Extract([]byte(`{"message":"ok","bad":[}`), "message", Options{})
	if err == nil || values != nil {
		t.Fatal("partial extraction returned on invalid input")
	}
}

func TestUnescapeExactlyOnce(t *testing.T) {
	if _, err := DecodeString([]byte(`'a'b'`), true); err == nil {
		t.Fatal("unescaped single quote accepted")
	}
	for _, tc := range []struct{ raw, want string }{
		{`"A\nB"`, "A\nB"}, {`"A\\nB"`, `A\nB`},
		{`"C:\\new\\test.txt"`, `C:\new\test.txt`},
		{`"\"quoted\" \/ \\ \b\f\r\t"`, "\"quoted\" / \\ \b\f\r\t"},
		{`'it\'s "ok"\n'`, "it's \"ok\"\n"},
	} {
		got, err := DecodeString([]byte(tc.raw), true)
		if err != nil || got != tc.want {
			t.Fatalf("%s => %q, %v; want %q", tc.raw, got, err, tc.want)
		}
	}
}

func TestRejectsMalformed(t *testing.T) {
	for _, src := range []string{"", " ", `{"x":}`, `{"x":1,}`, `[1,]`, `{"x" 1}`, `[}`, `01`, `-.2`, `1.`, `1e+`, `truefalse`, `{}[]`, `"unterminated`, `"bad\q"`, `"bad\u12xz"`, "\"a\nb\"", `{bare:1}`, `{'a':1}`, `/*no*/ null`, `[1 2]`, `[1`, "\x00", string([]byte{0xff})} {
		t.Run(src, func(t *testing.T) {
			out, err := Format([]byte(src), Options{})
			if err == nil {
				t.Fatalf("accepted %q as %s", src, out)
			}
		})
	}
	_, err := Format([]byte("{\n  \"x\": @}"), Options{})
	e, ok := err.(*SyntaxError)
	if !ok || e.Line != 2 || e.Column != 8 {
		t.Fatalf("location: %v", err)
	}
}

func TestDocumentsAndDepth(t *testing.T) {
	got, err := Format([]byte("\xef\xbb\xbf{\"x\":1}\n{\"x\":2}"), Options{Compact: true})
	if err != nil || string(got) != "{\"x\":1}\n{\"x\":2}" {
		t.Fatalf("%s %v", got, err)
	}
	values, err := Extract(got, "x", Options{})
	if err != nil || len(values) != 2 {
		t.Fatalf("%v %v", values, err)
	}
	_, err = Format([]byte(`[[[]]]`), Options{MaxDepth: 2})
	if err == nil {
		t.Fatal("depth exceeded")
	}
	_, err = Format([]byte(`[[[]]]`), Options{MaxDepth: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []Options{{Indent: -1}, {Indent: 9}, {MaxDepth: -1}, {MaxDepth: 4097}} {
		if _, err := Format([]byte("null"), opts); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
}

func TestLoose(t *testing.T) {
	src := []byte("// heading\n{bare: 'it\\'s fine', nested:[1,2,], /* comment */ last: true,}")
	for _, compact := range []bool{false, true} {
		opts := Options{Loose: true, Compact: compact}
		got, err := Format(src, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte("// heading\n")) || !bytes.Contains(got, []byte("/* comment */")) {
			t.Fatalf("comment lost: %s", got)
		}
		if _, err = Format(got, opts); err != nil {
			t.Fatalf("invalid relaxed output %s: %v", got, err)
		}
		values, err := Extract(got, "bare", opts)
		if err != nil || len(values) != 1 {
			t.Fatal(err)
		}
		v, err := DecodeString(values[0].Raw, true)
		if err != nil || v != "it's fine" {
			t.Fatalf("%s %v", v, err)
		}
	}
	if _, err := Format([]byte(`{/*`), Options{Loose: true}); err == nil {
		t.Fatal("unfinished comment accepted")
	}
}

func TestAppendRollback(t *testing.T) {
	dst := make([]byte, 6, 100)
	copy(dst, "prefix")
	got, err := AppendFormat(dst, []byte(`{"x":}`), Options{})
	if err == nil || string(got) != "prefix" {
		t.Fatalf("%q %v", got, err)
	}
}

func FuzzFormat(f *testing.F) {
	for _, s := range []string{`{"amount":1.2300,"x":1,"x":2}`, `["a\n",null,1E+12]`, `"\ud83d\ude3a"`, `{"":[]}`, `true`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 100000 || !utf8.ValidString(s) {
			return
		}
		out, err := Format([]byte(s), Options{})
		if err != nil {
			if json.Valid([]byte(s)) && !strings.Contains(err.Error(), "maximum nesting") {
				t.Fatalf("valid input rejected %q: %v", s, err)
			}
			return
		}
		if !json.Valid([]byte(s)) {
			return
		} // Multi-document and BOM inputs are supported too.
		if !json.Valid(out) {
			t.Fatalf("invalid output %q from %q", out, s)
		}
		var a, b bytes.Buffer
		json.Compact(&a, []byte(s))
		json.Compact(&b, out)
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatalf("changed tokens: %q != %q", &a, &b)
		}
	})
}

var benchmarkOutput []byte

func BenchmarkFormat(b *testing.B) {
	for _, n := range []int{1, 10000} {
		src := []byte(strings.TrimSpace(strings.Repeat(`{"log.level":"debug","amount":1.2300,"id":9007199254740993,"message":"\nREQUEST\nHEADERS:\n\tAccept: application/json\nBODY:\nNO CONTENT\n"}`+"\n", n)))
		b.Run(strconvLabel(n), func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkOutput, err = AppendFormat(benchmarkOutput[:0], src, Options{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func strconvLabel(n int) string {
	if n == 1 {
		return "single_log"
	}
	return "10000_logs"
}
func BenchmarkExtract(b *testing.B) {
	src := []byte(`{"log.level":"debug","message":"\nREQUEST\nHEADERS:\n\tAccept: application/json\n"}`)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := Extract(src, "message", Options{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatFreshBuffer(b *testing.B) {
	src := []byte(strings.TrimSpace(strings.Repeat(`{"log.level":"debug","amount":1.2300,"id":9007199254740993,"message":"\nREQUEST\nHEADERS:\n\tAccept: application/json\nBODY:\nNO CONTENT\n"}`+"\n", 10000)))
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkOutput, err = Format(src, Options{})
		if err != nil {
			b.Fatal(err)
		}
	}
}
