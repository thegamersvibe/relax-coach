package main

// Minimal parser for WoW SavedVariables files (Lua table literals only, never executed).

import (
	"fmt"
	"strconv"
	"strings"
)

type luaParser struct {
	s     string
	pos   int
	depth int
	// maxValues > 0 bounds the number of table entries parsed (the Farming
	// page uses it; the Coach save keeps the unbounded behaviour).
	maxValues, values int
}

func ParseSavedVariables(src string) (out map[string]any, err error) {
	defer func() { // belt and braces: a malformed file must never kill the app
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("SavedVariables: unreadable (%v)", r)
		}
	}()
	return parseSV(src, 0)
}

// ParseSavedVariablesLimited is ParseSavedVariables with at most maxValues
// table entries; more is an error, found before the work grows further.
func ParseSavedVariablesLimited(src string, maxValues int) (out map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("SavedVariables: unreadable (%v)", r)
		}
	}()
	return parseSV(src, maxValues)
}

func parseSV(src string, maxValues int) (map[string]any, error) {
	p := &luaParser{s: src, maxValues: maxValues}
	out := map[string]any{}
	for {
		p.skip()
		if p.pos >= len(p.s) {
			return out, nil
		}
		name := p.ident()
		if name == "" {
			return nil, p.err("expected variable name")
		}
		p.skip()
		if !p.eat('=') {
			return nil, p.err("expected =")
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out[name] = v
	}
}

func (p *luaParser) err(msg string) error {
	line := strings.Count(p.s[:min(p.pos, len(p.s))], "\n") + 1
	return fmt.Errorf("SavedVariables line %d: %s", line, msg)
}

func (p *luaParser) skip() {
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			p.pos++
		} else if strings.HasPrefix(p.s[p.pos:], "--") {
			if strings.HasPrefix(p.s[p.pos:], "--[[") {
				end := strings.Index(p.s[p.pos:], "]]")
				if end < 0 {
					p.pos = len(p.s)
				} else {
					p.pos += end + 2
				}
			} else {
				for p.pos < len(p.s) && p.s[p.pos] != '\n' {
					p.pos++
				}
			}
		} else {
			return
		}
	}
}

func (p *luaParser) eat(c byte) bool {
	p.skip()
	if p.pos < len(p.s) && p.s[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *luaParser) ident() string {
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || (p.pos > start && c >= '0' && c <= '9') {
			p.pos++
		} else {
			break
		}
	}
	return p.s[start:p.pos]
}

func (p *luaParser) value() (any, error) {
	p.skip()
	if p.pos >= len(p.s) {
		return nil, p.err("unexpected end")
	}
	c := p.s[p.pos]
	switch {
	case c == '{':
		return p.table()
	case c == '"' || c == '\'':
		return p.str()
	case c == '-' || c == '.' || c >= '0' && c <= '9':
		return p.num()
	}
	id := p.ident()
	switch id {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "nil":
		return nil, nil
	}
	return nil, p.err("unexpected token " + id)
}

func (p *luaParser) num() (any, error) {
	start := p.pos
	for p.pos < len(p.s) && strings.IndexByte("0123456789+-.eExXabcdefABCDEF", p.s[p.pos]) >= 0 {
		p.pos++
	}
	t := p.s[start:p.pos]
	if strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X") {
		i, err := strconv.ParseInt(t[2:], 16, 64)
		return float64(i), err
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return nil, p.err("bad number " + t)
	}
	return f, nil
}

func (p *luaParser) str() (any, error) {
	q := p.s[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == q {
			p.pos++
			return b.String(), nil
		}
		if c == '\\' && p.pos+1 < len(p.s) {
			p.pos++
			e := p.s[p.pos]
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\n':
				b.WriteByte('\n')
			case '\\', '"', '\'':
				b.WriteByte(e)
			default:
				if e >= '0' && e <= '9' {
					j := p.pos
					for j < len(p.s) && j < p.pos+3 && p.s[j] >= '0' && p.s[j] <= '9' {
						j++
					}
					n, _ := strconv.Atoi(p.s[p.pos:j])
					b.WriteByte(byte(n))
					p.pos = j - 1
				} else {
					b.WriteByte(e)
				}
			}
			p.pos++
			continue
		}
		b.WriteByte(c)
		p.pos++
	}
	return nil, p.err("unterminated string")
}

// Tables become map[string]any; array entries use keys "1", "2", ...
func (p *luaParser) table() (any, error) {
	p.pos++ // {
	out := map[string]any{}
	n := 0
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 64 {
		return nil, p.err("nesting too deep")
	}
	for {
		p.skip()
		if p.pos >= len(p.s) {
			return nil, p.err("unexpected end (file still being written?)")
		}
		if p.eat('}') {
			return out, nil
		}
		var key string
		if p.s[p.pos] == '[' {
			p.pos++
			k, err := p.value()
			if err != nil {
				return nil, err
			}
			if !p.eat(']') || !p.eat('=') {
				return nil, p.err("expected ] =")
			}
			switch kv := k.(type) {
			case string:
				key = kv
			case float64:
				key = strconv.FormatFloat(kv, 'f', -1, 64)
			default:
				key = fmt.Sprint(kv)
			}
		} else {
			save := p.pos
			id := p.ident()
			p.skip()
			if id != "" && id != "true" && id != "false" && id != "nil" && p.pos < len(p.s) && p.s[p.pos] == '=' {
				p.pos++
				key = id
			} else {
				p.pos = save
				n++
				key = strconv.Itoa(n)
			}
		}
		if p.values++; p.maxValues > 0 && p.values > p.maxValues {
			return nil, fmt.Errorf("more than %d values; not read", p.maxValues)
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out[key] = v
		p.skip()
		if !p.eat(',') && !p.eat(';') {
			if p.eat('}') {
				return out, nil
			}
			return nil, p.err("expected , or }")
		}
	}
}

// Helpers for navigating parsed data.
func M(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func S(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func N(v any) float64 {
	f, _ := v.(float64)
	return f
}

func List(v any) []any {
	m := M(v)
	var out []any
	for i := 1; ; i++ {
		x, ok := m[strconv.Itoa(i)]
		if !ok {
			return out
		}
		out = append(out, x)
	}
}
