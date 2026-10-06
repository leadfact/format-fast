// Package cli adapts terminal I/O to the reusable formatter package.
package cli

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"localformat/formatter"
)

const Version = "0.2.0"

// Run makes CLI behavior testable without replacing process-wide stdin/stdout.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("localformat", flag.ContinueOnError)
	f.SetOutput(stderr)
	textInput := f.String("text", "", "format inline JSON text explicitly")
	fileInput := f.String("file", "", "read a file explicitly (- for stdin)")
	indent := f.Int("indent", 2, "spaces per level (1..8)")
	compact := f.Bool("compact", false, "remove whitespace outside strings")
	loose := f.Bool("loose", false, "accept comments, single quotes, bare keys and trailing commas; preserve their syntax")
	extract := f.String("extract", "", "extract a literal key or JSON Pointer, e.g. message or /events/0/message")
	asJSON := f.Bool("json", false, "keep extracted strings JSON-quoted instead of decoding them")
	unescape := f.Bool("unescape", false, "decode an input consisting of JSON string(s), exactly one escape layer")
	stream := f.Bool("stream", false, "process NDJSON one line at a time; earlier lines remain on stdout if a later line fails")
	color := f.String("color", "auto", "syntax colors: auto, always, never (formatting only)")
	output := f.String("output", "", "write atomically to a file instead of stdout")
	maxBytes := f.Int64("max-bytes", 64<<20, "maximum input bytes (or bytes per line with -stream)")
	maxDepth := f.Int("max-depth", 512, "maximum nesting depth (1..4096)")
	version := f.Bool("version", false, "print version")
	f.Usage = func() {
		fmt.Fprint(stderr, "Usage: localformat [flags] [JSON|file|-]\n\nFormats JSON/NDJSON locally. No network, dependencies or telemetry.\nFlags may appear before or after the input. Use -- before a dash-prefixed filename.\nQuote inline JSON with shell single quotes. No input argument reads stdin.\n\nExamples:\n  localformat '{\"message\":\"hello\\nworld\"}'\n  localformat '{\"message\":\"hello\\nworld\"}' --extract message\n  localformat log.json\n  localformat --text '{\"x\":1}'\n  localformat --file log.json\n  cat log.json | localformat\n  tail -f app.jsonl | localformat --stream --extract message\n")
		f.PrintDefaults()
	}
	ordered, err := orderArgs(f, args)
	if err != nil {
		fmt.Fprintln(stderr, "localformat:", err)
		return 2
	}
	if err := f.Parse(ordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *version {
		fmt.Fprintln(stdout, "localformat", Version)
		return 0
	}
	errOut := func(err error) int { fmt.Fprintln(stderr, "localformat:", err); return 1 }
	if *maxBytes < 1 || *maxBytes > 1<<30 {
		return errOut(errors.New("max-bytes must be between 1 and 1073741824"))
	}
	if *indent < 1 || *indent > 8 {
		return errOut(errors.New("indent must be between 1 and 8"))
	}
	if *maxDepth < 1 || *maxDepth > 4096 {
		return errOut(errors.New("max-depth must be between 1 and 4096"))
	}
	if *color != "auto" && *color != "always" && *color != "never" {
		return errOut(errors.New("color must be auto, always or never"))
	}
	hasExtract := false
	hasText, hasFile := false, false
	f.Visit(func(v *flag.Flag) {
		switch v.Name {
		case "extract":
			hasExtract = true
		case "text":
			hasText = true
		case "file":
			hasFile = true
		}
	})
	if *unescape && hasExtract {
		return errOut(errors.New("use either -extract or -unescape"))
	}
	if *asJSON && !hasExtract {
		return errOut(errors.New("-json requires -extract"))
	}
	opts := formatter.Options{Indent: *indent, Compact: *compact, Loose: *loose, MaxDepth: *maxDepth}
	// Check option ranges even for an empty stream.
	if _, err := formatter.Format([]byte("null"), opts); err != nil {
		return errOut(err)
	}
	input, closer, err := openInput(stdin, inputSpec{args: f.Args(), text: *textInput, file: *fileInput, hasText: hasText, hasFile: hasFile})
	if err != nil {
		return errOut(err)
	}
	if closer != nil {
		defer closer.Close()
	}
	useColor := *color == "always" || (*color == "auto" && *output == "" && isTerminal(stdout))
	destination := stdout
	var temp *os.File
	if *output != "" {
		var err error
		temp, err = os.CreateTemp(filepath.Dir(*output), ".localformat-*")
		if err != nil {
			return errOut(err)
		}
		defer os.Remove(temp.Name())
		defer temp.Close()
		destination = temp
	}
	process := func(src []byte) error {
		var result []byte
		if hasExtract || *unescape {
			pointer := *extract
			if *unescape {
				pointer = ""
			}
			values, err := formatter.Extract(src, pointer, opts)
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return fmt.Errorf("field or pointer %q was not found", pointer)
			}
			for _, v := range values {
				if *unescape && !v.IsString {
					return errors.New("-unescape expects JSON strings")
				}
				if v.IsString && !*asJSON {
					s, err := formatter.DecodeString(v.Raw, *loose)
					if err != nil {
						return err
					}
					result = append(result, s...)
				} else {
					var err error
					result, err = formatter.AppendFormat(result, v.Raw, opts)
					if err != nil {
						return err
					}
				}
				if len(result) == 0 || result[len(result)-1] != '\n' {
					result = append(result, '\n')
				}
			}
		} else {
			var err error
			result, err = formatter.Format(src, opts)
			if err != nil {
				return err
			}
			if len(result) == 0 || result[len(result)-1] != '\n' {
				result = append(result, '\n')
			}
			if useColor {
				result = colorize(result)
			}
		}
		_, err := destination.Write(result)
		return err
	}
	if *stream {
		err = streamLines(input, *maxBytes, process)
	} else {
		var src []byte
		src, err = io.ReadAll(io.LimitReader(input, *maxBytes+1))
		if err == nil && int64(len(src)) > *maxBytes {
			err = fmt.Errorf("input exceeds %d bytes", *maxBytes)
		}
		if err == nil {
			err = process(src)
		}
	}
	if err != nil {
		return errOut(err)
	}
	if temp != nil {
		if err = temp.Close(); err != nil {
			return errOut(err)
		}
		if err = os.Rename(temp.Name(), *output); err != nil {
			return errOut(err)
		}
	}
	return 0
}

func isTerminal(w io.Writer) bool {
	if _, exists := os.LookupEnv("NO_COLOR"); exists || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	s, err := f.Stat()
	return err == nil && s.Mode()&os.ModeCharDevice != 0
}

func streamLines(r io.Reader, limit int64, process func([]byte) error) error {
	reader := bufio.NewReaderSize(r, 64<<10)
	var line []byte
	for n := 1; ; {
		part, err := reader.ReadSlice('\n')
		if int64(len(line))+int64(len(part)) > limit {
			return fmt.Errorf("NDJSON line %d exceeds %d bytes", n, limit)
		}
		line = append(line, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return err
		}
		if len(bytes.TrimSpace(line)) > 0 {
			if e := process(line); e != nil {
				return fmt.Errorf("NDJSON line %d: %w", n, e)
			}
		}
		line = line[:0]
		n++
		if err == io.EOF {
			return nil
		}
	}
}

// Colors are an optional presentation layer and never affect the core parser.
func colorize(src []byte) []byte {
	out := make([]byte, 0, len(src)+len(src)/3)
	for i := 0; i < len(src); {
		start := i
		c := src[i]
		code := ""
		if c == '"' || c == '\'' {
			quote := c
			i++
			for i < len(src) {
				c = src[i]
				i++
				if c == '\\' && i < len(src) {
					i++
					continue
				}
				if c == quote {
					break
				}
			}
			j := i
			for j < len(src) && strings.ContainsRune(" \r\n\t", rune(src[j])) {
				j++
			}
			code = "\x1b[32m"
			if j < len(src) && src[j] == ':' {
				code = "\x1b[36m"
			}
		} else if c == '/' && i+1 < len(src) && (src[i+1] == '/' || src[i+1] == '*') {
			line := src[i+1] == '/'
			i += 2
			if line {
				for i < len(src) && src[i] != '\n' {
					i++
				}
			} else {
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < len(src) {
					i += 2
				}
			}
			code = "\x1b[90m"
		} else if c == '-' || c >= '0' && c <= '9' {
			i++
			for i < len(src) && strings.ContainsRune("0123456789.eE+-", rune(src[i])) {
				i++
			}
			code = "\x1b[33m"
		} else if c >= 'a' && c <= 'z' {
			i++
			for i < len(src) && src[i] >= 'a' && src[i] <= 'z' {
				i++
			}
			code = "\x1b[35m"
		} else {
			i++
		}
		if code != "" {
			out = append(out, code...)
		}
		out = append(out, src[start:i]...)
		if code != "" {
			out = append(out, "\x1b[0m"...)
		}
	}
	return out
}
