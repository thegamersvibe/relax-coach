package cfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

var contentVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func decode(b []byte, dst any) error {
	// encoding/json otherwise silently accepts duplicate keys and case-folded
	// field names. Neither is safe for a hand-edited scoring configuration.
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueKeys(d, b, "$"); err != nil {
		return err
	}
	value := b[:d.InputOffset()]
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("$: expected exactly one JSON value")
	}
	b = value // Shape/type checks consume the one value; the check above owns trailing data.
	if err := requiredShape(b, reflect.TypeOf(dst).Elem(), "$"); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(b))
	// requiredShape checks unknown fields with full JSON paths.
	if err := d.Decode(dst); err != nil {
		return err
	}
	var m Metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.Schema != 1 {
		return fmt.Errorf("unsupported schema %d (reader supports 1)", m.Schema)
	}
	if !contentVersion.MatchString(m.Version) {
		return fmt.Errorf("version must be a major.minor.patch content revision")
	}
	if strings.TrimSpace(m.Source) == "" {
		return fmt.Errorf("source note is required")
	}
	return nil
}

// Scan raw token bytes before encoding/json can replace malformed UTF-8.
func uniqueKeys(d *json.Decoder, b []byte, path string) error {
	start := d.InputOffset()
	t, err := d.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !utf8.Valid(b[start:d.InputOffset()]) {
		return fmt.Errorf("%s: invalid UTF-8", path)
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			start := d.InputOffset()
			key, err := d.Token()
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("%s: object key is not a string", path)
			}
			field := path + "." + name
			if !utf8.Valid(b[start:d.InputOffset()]) {
				return fmt.Errorf("%s: invalid UTF-8 in object key", field)
			}
			if seen[normalized(name)] {
				return fmt.Errorf("%s: duplicate key %q", field, name)
			}
			seen[normalized(name)] = true
			if err := uniqueKeys(d, b, field); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for i := 0; d.More(); i++ {
			if err := uniqueKeys(d, b, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Require explicit values, including false/zero/empty arrays. Only pointers
// accept null; omitempty permits absent spell ids and optional selector fields.
// Unknown fields and container-shape errors retain their full JSON path;
// the final typed decode supplies field names for scalar-type errors.
func requiredShape(b []byte, typ reflect.Type, path string) error {
	if typ.Kind() == reflect.Pointer {
		if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
			return nil
		}
		return requiredShape(b, typ.Elem(), path)
	}
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return fmt.Errorf("%s cannot be null", path)
	}
	switch typ.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(b, &fields); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		known := jsonFields(typ)
		for key := range fields {
			if _, ok := known[key]; !ok {
				return fmt.Errorf("%s.%s: unknown field (keys are case sensitive)", path, key)
			}
		}
		for name, f := range known {
			value, exists := fields[name]
			if !exists {
				if strings.Contains(f.Tag.Get("json"), ",omitempty") {
					continue
				}
				return fmt.Errorf("%s.%s is required (keys are case sensitive)", path, name)
			}
			if err := requiredShape(value, f.Type, path+"."+name); err != nil {
				return err
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if err := json.Unmarshal(b, &values); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for i, v := range values {
			if err := requiredShape(v, typ.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		var values map[string]json.RawMessage
		if err := json.Unmarshal(b, &values); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for name, v := range values {
			if err := requiredShape(v, typ.Elem(), path+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}

func jsonFields(typ reflect.Type) map[string]reflect.StructField {
	fields := map[string]reflect.StructField{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Anonymous {
			for name, inner := range jsonFields(f.Type) {
				fields[name] = inner
			}
			continue
		}
		fields[strings.Split(f.Tag.Get("json"), ",")[0]] = f
	}
	return fields
}
