package cfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
)

var contentVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func decode(b []byte, dst any) error {
	// encoding/json otherwise silently accepts duplicate keys and case-folded
	// field names. Neither is safe for a hand-edited scoring configuration.
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueKeys(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	if err := requiredShape(b, reflect.TypeOf(dst).Elem(), "$"); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
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

func uniqueKeys(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if seen[name] {
				return fmt.Errorf("duplicate key %q", name)
			}
			seen[name] = true
			if err := uniqueKeys(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for d.More() {
			if err := uniqueKeys(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

// Require explicit values, including false/zero/empty arrays. Only pointers
// accept null; omitempty permits absent spell ids and optional selector fields.
// Unknown fields are rejected by the standard decoder after this shape pass.
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
			return err
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.Anonymous {
				if err := requiredShape(b, f.Type, path); err != nil {
					return err
				}
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			for key := range fields {
				if key != name && strings.EqualFold(key, name) {
					return fmt.Errorf("%s.%s is a case-folded alias; use only %s", path, key, name)
				}
			}
			value, exists := fields[name]
			if !exists {
				if len(tag) > 1 && tag[1] == "omitempty" {
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
			return err
		}
		for i, v := range values {
			if err := requiredShape(v, typ.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		var values map[string]json.RawMessage
		if err := json.Unmarshal(b, &values); err != nil {
			return err
		}
		for name, v := range values {
			if err := requiredShape(v, typ.Elem(), path+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}
