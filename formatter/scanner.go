package formatter

// The scanner advances over source tokens without converting their values.
func isBareKeyByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '$' || c == '.' || c == '-' || c == '@'
}
func isHex(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
func (p *parser) stringToken() error {
	quote := p.peek()
	p.i++
	for p.i < len(p.src) {
		c := p.src[p.i]
		p.i++
		if c == quote {
			return nil
		}
		if c < 0x20 {
			p.i--
			return p.fail("unescaped control character in string")
		}
		if c != '\\' {
			continue
		}
		if p.i == len(p.src) {
			return p.fail("unfinished escape sequence")
		}
		c = p.src[p.i]
		p.i++
		switch c {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		case '\'':
			if !p.loose || quote != '\'' {
				return p.fail("invalid escape sequence")
			}
		case 'u':
			for n := 0; n < 4; n++ {
				if p.i == len(p.src) || !isHex(p.src[p.i]) {
					return p.fail("expected four hexadecimal digits after \\u")
				}
				p.i++
			}
		default:
			return p.fail("invalid escape sequence")
		}
	}
	return p.fail("unterminated string")
}
func (p *parser) number() error {
	if p.peek() == '-' {
		p.i++
	}
	c := p.peek()
	if c == '0' {
		p.i++
	} else if c >= '1' && c <= '9' {
		for p.peek() >= '0' && p.peek() <= '9' {
			p.i++
		}
	} else {
		return p.fail("expected digit")
	}
	if p.peek() == '.' {
		p.i++
		if p.peek() < '0' || p.peek() > '9' {
			return p.fail("expected digit after decimal point")
		}
		for p.peek() >= '0' && p.peek() <= '9' {
			p.i++
		}
	}
	if p.peek() == 'e' || p.peek() == 'E' {
		p.i++
		if p.peek() == '+' || p.peek() == '-' {
			p.i++
		}
		if p.peek() < '0' || p.peek() > '9' {
			return p.fail("expected exponent digits")
		}
		for p.peek() >= '0' && p.peek() <= '9' {
			p.i++
		}
	}
	return nil
}
