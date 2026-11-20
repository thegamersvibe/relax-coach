package logfmt

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed limits.json
var limitsJSON []byte
var limits = func() struct{ MaxLineBytes int } {
	var v struct{ MaxLineBytes int }
	if err := json.Unmarshal(limitsJSON, &v); err != nil || v.MaxLineBytes <= 0 {
		panic("invalid embedded decoder limits")
	}
	return v
}()

// readLine drains oversize input without growing the retained line. n always
// includes every consumed byte, including CRLF and discarded fragments.
func readLine(r *bufio.Reader) (raw string, n int64, err error) {
	var b strings.Builder
	for {
		part, e := r.ReadSlice('\n')
		n += int64(len(part))
		if n <= int64(limits.MaxLineBytes) {
			b.Write(part)
		} else {
			b.Reset()
		}
		if e != bufio.ErrBufferFull {
			if n > int64(limits.MaxLineBytes) {
				return "", n, e
			}
			return b.String(), n, e
		}
	}
}
