package logfmt

import "strings"

// split recognizes both list delimiters, so commas in combatant lists and
// quoted names do not alter the top-level field count. Nested text stays raw.
func split(s string) ([]string, bool) {
	var fields []string
	var stack []byte
	quoted, escaped := false, false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				quoted = false
			}
			continue
		}
		switch c {
		case '"':
			quoted = true
		case '[', '(':
			stack = append(stack, c)
		case ']', ')':
			if len(stack) == 0 {
				return nil, false
			}
			open := stack[len(stack)-1]
			if (c == ']' && open != '[') || (c == ')' && open != '(') {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				fields = append(fields, s[start:i])
				start = i + 1
			}
		}
	}
	if quoted || escaped || len(stack) != 0 {
		return nil, false
	}
	fields = append(fields, s[start:])
	for i, f := range fields {
		f = strings.TrimSpace(f)
		if strings.HasPrefix(f, `"`) {
			if len(f) < 2 || f[len(f)-1] != '"' {
				return nil, false
			}
			var b strings.Builder
			for j := 1; j < len(f)-1; j++ {
				c := f[j]
				if c == '\\' {
					j++
					if j >= len(f)-1 {
						return nil, false
					}
					switch f[j] {
					case '\\', '"':
						c = f[j]
					case 'n':
						c = '\n'
					case 'r':
						c = '\r'
					case 't':
						c = '\t'
					default:
						return nil, false
					}
				} else if c == '"' {
					return nil, false
				}
				b.WriteByte(c)
			}
			f = b.String()
		}
		fields[i] = f
	}
	return fields, true
}
