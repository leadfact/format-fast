package formatter

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode/utf8"
)

type parser struct {
	src, out                        []byte
	i, depth, indent, maxDepth      int
	compact, loose, emit, selecting bool
	target                          []string
	matches                         []Value
}

func newParser(src []byte, opts Options) (parser, error) {
	if opts.Indent == 0 {
		opts.Indent = 2
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 512
	}
	if opts.Indent < 1 || opts.Indent > 8 {
		return parser{}, fmt.Errorf("indent must be between 1 and 8")
	}
	if opts.MaxDepth < 1 || opts.MaxDepth > 4096 {
		return parser{}, fmt.Errorf("max depth must be between 1 and 4096")
	}
	p := parser{src: src, indent: opts.Indent, maxDepth: opts.MaxDepth, compact: opts.Compact, loose: opts.Loose}
	if !utf8.Valid(src) {
		return parser{}, fmt.Errorf("input is not valid UTF-8")
	}
	if bytes.HasPrefix(src, []byte{0xef, 0xbb, 0xbf}) {
		p.i = 3
	}
	return p, nil
}

func (p *parser) fail(message string) error {
	line, column := 1, 1
	for _, c := range p.src[:p.i] {
		if c == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return &SyntaxError{Offset: p.i, Line: line, Column: column, Message: message}
}
func (p *parser) char(c byte) {
	if p.emit {
		p.out = append(p.out, c)
	}
}
func (p *parser) raw(start, end int) {
	if p.emit {
		p.out = append(p.out, p.src[start:end]...)
	}
}
func (p *parser) newline() {
	if !p.emit || p.compact {
		return
	}
	// Reuse an existing blank line (e.g. after a relaxed trailing comma),
	// adjusting its indentation instead of inserting another empty line.
	end := len(p.out)
	for end > 0 && (p.out[end-1] == ' ' || p.out[end-1] == '\t') {
		end--
	}
	if end > 0 && p.out[end-1] == '\n' {
		p.out = p.out[:end]
	} else {
		p.out = append(p.out, '\n')
	}
	for n := 0; n < p.depth*p.indent; n++ {
		p.out = append(p.out, ' ')
	}
}
func (p *parser) peek() byte {
	if p.i < len(p.src) {
		return p.src[p.i]
	}
	return 0
}

// skip also preserves comments when accepting relaxed JSON. A newline after a
// line comment is mandatory even in compact mode, otherwise it eats the value.
func (p *parser) skip() error {
	for p.i < len(p.src) {
		switch p.src[p.i] {
		case ' ', '\t', '\r', '\n':
			p.i++
			continue
		case '/':
			if !p.loose || p.i+1 >= len(p.src) {
				return nil
			}
			start := p.i
			if p.src[p.i+1] == '/' {
				p.i += 2
				for p.i < len(p.src) && p.src[p.i] != '\n' {
					p.i++
				}
				p.raw(start, p.i)
				p.char('\n')
				if p.emit && !p.compact {
					for n := 0; n < p.depth*p.indent; n++ {
						p.char(' ')
					}
				}
				continue
			}
			if p.src[p.i+1] == '*' {
				p.i += 2
				end := bytes.Index(p.src[p.i:], []byte("*/"))
				if end < 0 {
					return p.fail("unterminated block comment")
				}
				p.i += end + 2
				p.raw(start, p.i)
				p.char(' ')
				continue
			}
		}
		return nil
	}
	return nil
}

func (p *parser) documents() error {
	if err := p.skip(); err != nil {
		return err
	}
	if p.i == len(p.src) {
		return p.fail("expected JSON input")
	}
	for {
		if err := p.value(0); err != nil {
			return err
		}
		end := p.i
		if err := p.skip(); err != nil {
			return err
		}
		if p.i == len(p.src) {
			return nil
		}
		if p.i == end {
			return p.fail("expected whitespace between JSON documents")
		}
		p.char('\n')
	}
}

// matchDepth is the matched pointer prefix, or -1 for an unrelated subtree.
func (p *parser) value(matchDepth int) error {
	if err := p.skip(); err != nil {
		return err
	}
	start := p.i
	c := p.peek()
	if c == '{' || c == '[' {
		if p.depth >= p.maxDepth {
			return p.fail("maximum nesting depth exceeded")
		}
		if err := p.container(c, matchDepth); err != nil {
			return err
		}
	} else {
		if c == '"' || (p.loose && c == '\'') {
			if err := p.stringToken(); err != nil {
				return err
			}
		} else if c == '-' || (c >= '0' && c <= '9') {
			if err := p.number(); err != nil {
				return err
			}
		} else {
			literal := ""
			switch c {
			case 't':
				literal = "true"
			case 'f':
				literal = "false"
			case 'n':
				literal = "null"
			}
			if literal == "" || !bytes.HasPrefix(p.src[p.i:], []byte(literal)) {
				return p.fail("expected a JSON value")
			}
			p.i += len(literal)
		}
		p.raw(start, p.i)
	}
	if p.selecting && matchDepth == len(p.target) {
		p.matches = append(p.matches, Value{Raw: p.src[start:p.i], IsString: c == '"' || c == '\''})
	}
	return nil
}

func (p *parser) childMatch(parent int, key string) int {
	if p.selecting && parent >= 0 && parent < len(p.target) && p.target[parent] == key {
		return parent + 1
	}
	return -1
}

func (p *parser) container(open byte, matchDepth int) error {
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	p.char(open)
	p.i++
	p.depth++
	if err := p.skip(); err != nil {
		return err
	}
	if p.peek() == close {
		p.i++
		p.depth--
		p.char(close)
		return nil
	}
	p.newline()
	for index := 0; ; index++ {
		child := -1
		if open == '{' {
			start := p.i
			quoted := p.peek() == '"' || (p.loose && p.peek() == '\'')
			if quoted {
				if err := p.stringToken(); err != nil {
					return err
				}
			} else if p.loose {
				for p.i < len(p.src) && isBareKeyByte(p.src[p.i]) {
					p.i++
				}
				if start == p.i {
					return p.fail("expected an object key")
				}
			} else {
				return p.fail("expected a double-quoted object key")
			}
			p.raw(start, p.i)
			if p.selecting && matchDepth >= 0 && matchDepth < len(p.target) {
				raw := p.src[start:p.i]
				if quoted && bytes.IndexByte(raw, '\\') >= 0 {
					key, err := DecodeString(raw, p.loose)
					if err != nil {
						return err
					}
					child = p.childMatch(matchDepth, key)
				} else {
					if quoted {
						raw = raw[1 : len(raw)-1]
					}
					// Comparing a temporary string avoids allocating decoded keys.
					if string(raw) == p.target[matchDepth] {
						child = matchDepth + 1
					}
				}
			}
			if err := p.skip(); err != nil {
				return err
			}
			if p.peek() != ':' {
				return p.fail("expected ':' after object key")
			}
			p.i++
			p.char(':')
			if !p.compact {
				p.char(' ')
			}
		} else if p.selecting {
			child = p.childMatch(matchDepth, strconv.Itoa(index))
		}
		if err := p.value(child); err != nil {
			return err
		}
		if err := p.skip(); err != nil {
			return err
		}
		if p.peek() == close {
			p.i++
			p.depth--
			p.newline()
			p.char(close)
			return nil
		}
		if p.peek() != ',' {
			return p.fail(fmt.Sprintf("expected ',' or '%c'", close))
		}
		p.i++
		p.char(',')
		p.newline()
		if err := p.skip(); err != nil {
			return err
		}
		if p.peek() == close {
			if !p.loose {
				return p.fail("trailing comma is not valid JSON")
			}
			p.i++
			p.depth--
			p.newline()
			p.char(close)
			return nil
		}
	}
}
