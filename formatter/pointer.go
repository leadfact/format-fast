package formatter

import (
	"fmt"
	"strings"
)

func parsePointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if pointer[0] != '/' {
		return []string{pointer}, nil
	}
	parts := strings.Split(pointer[1:], "/")
	for i, s := range parts {
		var b strings.Builder
		for n := 0; n < len(s); n++ {
			if s[n] != '~' {
				b.WriteByte(s[n])
				continue
			}
			n++
			if n == len(s) || (s[n] != '0' && s[n] != '1') {
				return nil, fmt.Errorf("invalid JSON Pointer escape")
			}
			if s[n] == '0' {
				b.WriteByte('~')
			} else {
				b.WriteByte('/')
			}
		}
		parts[i] = b.String()
	}
	return parts, nil
}
