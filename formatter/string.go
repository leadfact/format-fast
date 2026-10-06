package formatter

import (
	"encoding/json"
	"fmt"
)

// DecodeString decodes exactly one layer of string escaping. In strict mode it
// follows encoding/json semantics, including UTF-16 surrogate pairs.
func DecodeString(raw []byte, loose bool) (string, error) {
	if loose && len(raw) >= 2 && raw[0] == '\'' {
		if raw[len(raw)-1] != '\'' {
			return "", fmt.Errorf("unterminated single-quoted string")
		}
		// Translate only the quote syntax; keep JSON escape semantics.
		buf := make([]byte, 0, len(raw)+2)
		buf = append(buf, '"')
		for i := 1; i < len(raw)-1; i++ {
			c := raw[i]
			if c == '\\' && i+1 < len(raw)-1 {
				i++
				if raw[i] == '\'' {
					buf = append(buf, '\'')
				} else {
					buf = append(buf, '\\', raw[i])
				}
			} else if c == '\'' {
				return "", fmt.Errorf("unescaped quote inside single-quoted string")
			} else if c == '"' {
				buf = append(buf, '\\', '"')
			} else {
				buf = append(buf, c)
			}
		}
		buf = append(buf, '"')
		raw = buf
	}
	var value string
	if len(raw) == 0 || raw[0] != '"' {
		return "", fmt.Errorf("expected a quoted string")
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}
