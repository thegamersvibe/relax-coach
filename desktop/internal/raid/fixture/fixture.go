// Package fixture writes invented combat-log fixtures. It deliberately shares
// no tokenizer, layout table or formatting code with logfmt. Callers supply
// synthetic values; never use private log rows as fixture input.
package fixture

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Field is a wire token. Use Text for strings, Number for integers and the
// collection builders for nested values. Explicit Field values allow malformed
// fixtures without teaching the production decoder how to generate its input.
type Field string

const Nil Field = "nil"

// Text quotes a string using the reported backslash escaping convention.
func Text(s string) Field {
	return Field(`"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", `\r`, "\n", `\n`).Replace(s) + `"`)
}
func Number(n int64) Field { return Field(strconv.FormatInt(n, 10)) }
func join(fields []Field) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = string(f)
	}
	return strings.Join(parts, ",")
}
func List(fields ...Field) Field  { return Field("[" + join(fields) + "]") }
func Tuple(fields ...Field) Field { return Field("(" + join(fields) + ")") }

// Write writes one CRLF-terminated line, preserving the supplied wall time and
// whole-hour offset separately. It does not convert the wall time to UTC.
func Write(w io.Writer, at time.Time, offset int8, name string, fields ...Field) error {
	stamp := fmt.Sprintf("%d/%d/%d %d:%02d:%02d.%03d%d", at.Month(), at.Day(), at.Year(), at.Hour(), at.Minute(), at.Second(), at.Nanosecond()/int(time.Millisecond), offset)
	body := name
	if len(fields) > 0 {
		body += "," + join(fields)
	}
	row := stamp + "  " + body + "\r\n"
	n, err := io.WriteString(w, row)
	if err == nil && n != len(row) {
		return io.ErrShortWrite
	}
	return err
}
