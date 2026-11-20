package prepread

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// Key types stay distinct: [1] is never ["1"]. Duplicate assignments are
// refused rather than silently overwriting evidence, including nil entries.
type table map[any]any

type literalParser struct {
	s            string
	pos, entries int
	err          error
}

func (p *literalParser) fail(why string) {
	if p.err == nil {
		p.err = errors.New(why)
	}
}

func (p *literalParser) peek() byte {
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

// Only literal comments; an unterminated block is not a complete save.
func (p *literalParser) skip() {
	for p.pos < len(p.s) && p.err == nil {
		if strings.ContainsRune(" \t\r\n\v\f", rune(p.peek())) {
			p.pos++
			continue
		}
		if !strings.HasPrefix(p.s[p.pos:], "--") {
			return
		}
		p.pos += 2
		open := p.pos
		equals := 0
		if p.peek() == '[' {
			open++
			for open < len(p.s) && p.s[open] == '=' {
				open++
				equals++
				if equals > 64 {
					p.fail("literal syntax")
					return
				}
			}
		}
		if p.peek() == '[' && open < len(p.s) && p.s[open] == '[' {
			close := "]" + strings.Repeat("=", equals) + "]"
			end := strings.Index(p.s[open+1:], close)
			if end < 0 {
				p.fail("literal syntax")
				return
			}
			p.pos = open + 1 + end + len(close)
		} else {
			for p.pos < len(p.s) && p.peek() != '\n' {
				p.pos++
			}
		}
	}
}

func (p *literalParser) eat(c byte) bool {
	p.skip()
	if p.pos < len(p.s) && p.peek() == c {
		p.pos++
		return true
	}
	return false
}

func letter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
func digit(c byte) bool  { return c >= '0' && c <= '9' }

func (p *literalParser) ident() string {
	start := p.pos
	if letter(p.peek()) {
		p.pos++
		for letter(p.peek()) || digit(p.peek()) {
			p.pos++
		}
	}
	return p.s[start:p.pos]
}

func (p *literalParser) quoted() string {
	quote := p.peek()
	p.pos++
	var out strings.Builder
	for p.pos < len(p.s) && p.err == nil {
		c := p.peek()
		p.pos++
		if c == quote {
			return out.String()
		}
		if c == '\n' || c == '\r' {
			p.fail("literal syntax")
			break
		}
		if c == '\\' {
			if p.pos == len(p.s) {
				break
			}
			c = p.peek()
			p.pos++
			switch c {
			case 'a':
				c = '\a'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'v':
				c = '\v'
			case '\\', '\'', '"', '\n':
			case '\r':
				if p.peek() == '\n' {
					p.pos++
				}
				c = '\n'
			default:
				if !digit(c) {
					p.fail("literal syntax")
					break
				}
				n := int(c - '0')
				for i := 1; i < 3 && digit(p.peek()); i++ {
					n = n*10 + int(p.peek()-'0')
					p.pos++
				}
				if n > 255 {
					p.fail("literal syntax")
					break
				}
				c = byte(n)
			}
		}
		if out.Len() >= limits.MaxStringBytes {
			p.fail("literal string")
			break
		}
		out.WriteByte(c)
	}
	p.fail("literal syntax")
	return ""
}

func (p *literalParser) number() any {
	start := p.pos
	if p.peek() == '-' {
		p.pos++
	}
	digits := 0
	for digit(p.peek()) {
		p.pos++
		digits++
	}
	if p.peek() == '.' {
		p.pos++
		for digit(p.peek()) {
			p.pos++
			digits++
		}
	}
	if digits == 0 {
		p.fail("literal syntax")
		return nil
	}
	if p.peek() == 'e' || p.peek() == 'E' {
		p.pos++
		if p.peek() == '-' || p.peek() == '+' {
			p.pos++
		}
		exp := p.pos
		for digit(p.peek()) {
			p.pos++
		}
		if exp == p.pos {
			p.fail("literal syntax")
			return nil
		}
	}
	if p.pos-start > 64 {
		p.fail("literal syntax")
		return nil
	}
	v, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		p.fail("literal syntax")
		return nil
	}
	return v
}

func (p *literalParser) value(depth int) any {
	p.skip()
	if p.err != nil {
		return nil
	}
	switch p.peek() {
	case '{':
		return p.table(depth + 1)
	case '\'', '"':
		return p.quoted()
	case '-', '.':
		return p.number()
	}
	if digit(p.peek()) {
		return p.number()
	}
	switch p.ident() {
	case "true":
		return true
	case "false":
		return false
	case "nil":
		return nil
	}
	p.fail("literal syntax")
	return nil
}

func (p *literalParser) table(depth int) table {
	if depth > limits.MaxDepth {
		p.fail("literal depth")
		return nil
	}
	p.pos++
	out, next := table{}, 1
	for p.err == nil {
		p.skip()
		if p.eat('}') {
			return out
		}
		p.entries++
		if p.entries > limits.MaxEntries {
			p.fail("literal entries")
			break
		}
		var key any
		if p.eat('[') {
			key = p.value(depth)
			switch key.(type) {
			case string, float64, bool:
			default:
				p.fail("literal key")
			}
			if !p.eat(']') || !p.eat('=') {
				p.fail("literal syntax")
			}
		} else {
			at := p.pos
			id := p.ident()
			if id != "" && id != "true" && id != "false" && id != "nil" && p.eat('=') {
				if len(id) > limits.MaxStringBytes {
					p.fail("literal string")
				}
				key = id
			} else {
				p.pos = at
				key = float64(next)
				next++
			}
		}
		if p.err != nil {
			break
		}
		if _, duplicate := out[key]; duplicate {
			p.fail("duplicate key")
			break
		}
		out[key] = p.value(depth)
		if p.eat('}') {
			return out
		}
		if !p.eat(',') && !p.eat(';') {
			p.fail("literal syntax")
		}
	}
	return nil
}

func parseLiteral(data []byte) (table, error) {
	if len(data) > limits.MaxFileBytes {
		return nil, errors.New("file too large")
	}
	p := literalParser{s: string(data)}
	p.skip()
	if p.ident() != "RelaxRaidDB" || !p.eat('=') {
		return nil, errors.New("literal assignment")
	}
	v := p.value(0)
	p.eat(';')
	p.skip()
	if p.pos != len(p.s) {
		p.fail("literal syntax")
	}
	if p.err != nil {
		return nil, p.err
	}
	root, ok := v.(table)
	if !ok {
		return nil, errors.New("root type")
	}
	return root, nil
}
