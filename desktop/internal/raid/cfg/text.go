package cfg

import (
	"bytes"
	"strconv"
	"unicode/utf16"
)

// rawKeyPath is diagnostic-only: recover the authored key when Token refuses
// its syntax (for example an embedded raw NUL). Keep control bytes and escapes
// quoted, never repair the input or let a corrupt key put a raw NUL in an error.
func rawKeyPath(raw []byte, path string) string {
	raw = bytes.TrimLeft(raw, " 	\r\n,")
	if len(raw) == 0 || raw[0] != '"' {
		return path
	}
	for i := 1; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++ // An escaped quote is not the key terminator.
		case '"':
			key := string(raw[1:i])
			if len(key) > 64 {
				key = key[:64] + "..."
			}
			return path + "[" + strconv.Quote(key) + "]"
		}
	}
	return path
}

// validSurrogateEscapes examines an already syntactically valid JSON string
// token. encoding/json accepts unpaired UTF-16 escapes but silently replaces
// them; require a high/low pair instead. Escaped backslashes do not start Unicode escapes.
func validSurrogateEscapes(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		code, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if !utf16.IsSurrogate(rune(code)) {
			continue
		}
		// The low-surrogate half has bit 10 set; it cannot start a pair.
		if code>>10&1 != 0 {
			return false
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || !utf16.IsSurrogate(rune(low)) || low>>10&1 == 0 {
			return false
		}
		i += 6
	}
	return true
}
