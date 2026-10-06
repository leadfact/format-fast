// Package formatter formats JSON without converting numbers to floating point
// or objects to maps. Original scalar tokens and duplicate keys are preserved.
package formatter

import (
	"fmt"
	"slices"
)

// Options is independent of the terminal; the same API can serve a future UI.
type Options struct {
	Indent   int // Spaces per level, 0 defaults to 2; valid range 1..8.
	Compact  bool
	Loose    bool // Also accept comments, single quotes, bare keys and trailing commas.
	MaxDepth int  // 0 defaults to 512.
}

// SyntaxError identifies a zero-based byte offset and a one-based line and
// byte column (not a Unicode character or terminal display column).
type SyntaxError struct {
	Offset, Line, Column int
	Message              string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("line %d, column %d (byte %d): %s", e.Line, e.Column, e.Offset, e.Message)
}

// Value is a view into the source passed to Extract. Do not mutate the source
// while using returned values. Strings include their original quotes.
type Value struct {
	Raw      []byte
	IsString bool
}

// Format accepts one JSON document or multiple whitespace-separated documents
// (including NDJSON). It does not append a terminal newline.
func Format(src []byte, opts Options) ([]byte, error) { return AppendFormat(nil, src, opts) }

// AppendFormat appends formatted JSON to dst. On error, the original length of
// dst is returned; callers must disregard changes beyond that length.
// Source and destination must not overlap in memory.
func AppendFormat(dst, src []byte, opts Options) ([]byte, error) {
	p, err := newParser(src, opts)
	if err != nil {
		return dst, err
	}
	start := len(dst)
	// Most log objects grow modestly when indented. Reserve once, while allowing
	// callers with reusable buffers to avoid allocations entirely.
	reserve := len(src)
	if !opts.Compact {
		reserve += len(src)/4 + 64
	}
	p.out = slices.Grow(dst, reserve)
	p.emit = true
	if err = p.documents(); err != nil {
		return p.out[:start], err
	}
	return p.out, nil
}

// Extract selects a literal top-level key ("message") or an RFC 6901 JSON
// Pointer ("/events/0/message"). Empty pointer selects each whole document.
// It returns every match, including duplicate keys and matches across NDJSON.
// The whole input is validated before any values are returned.
func Extract(src []byte, pointer string, opts Options) ([]Value, error) {
	parts, err := parsePointer(pointer)
	if err != nil {
		return nil, err
	}
	p, err := newParser(src, opts)
	if err != nil {
		return nil, err
	}
	p.selecting, p.target = true, parts
	if err = p.documents(); err != nil {
		return nil, err
	}
	return p.matches, nil
}
