// Package compare builds transient officer comparisons. It performs no I/O and
// neither persists the officer's file nor changes any score.
package compare

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OfficerFile is the hand-written prediction, not part of a stored night.
// Top is best first; Bottom is worst first.
type OfficerFile struct {
	Version int      `json:"version"`
	Officer string   `json:"officer,omitempty"`
	Top     []string `json:"top"`
	Bottom  []string `json:"bottom"`
}

// ParseOfficer parses caller-selected bytes; the caller owns the path and I/O.
func ParseOfficer(data []byte) (OfficerFile, error) {
	if !validJSONText(data) {
		return OfficerFile{}, errors.New("invalid officer JSON or Unicode")
	}
	var out OfficerFile
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return OfficerFile{}, errors.New("officer file must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return OfficerFile{}, err
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return OfficerFile{}, errors.New("duplicate officer field")
		}
		seen[key] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return OfficerFile{}, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return OfficerFile{}, errors.New("null officer field")
		}
		switch key {
		case "version":
			err = json.Unmarshal(value, &out.Version)
		case "officer":
			err = json.Unmarshal(value, &out.Officer)
		case "top":
			err = json.Unmarshal(value, &out.Top)
		case "bottom":
			err = json.Unmarshal(value, &out.Bottom)
		default:
			return OfficerFile{}, errors.New("unknown officer field")
		}
		if err != nil {
			return OfficerFile{}, err
		}
	}
	if _, err := d.Token(); err != nil {
		return OfficerFile{}, err
	}
	if _, err := d.Token(); err != io.EOF {
		return OfficerFile{}, errors.New("trailing officer data")
	}
	if err := out.validate(); err != nil {
		return OfficerFile{}, err
	}
	return out, nil
}

func (f OfficerFile) validate() error {
	if f.Version != 1 {
		return errors.New("officer version must be 1")
	}
	seen := map[string]bool{}
	for _, list := range [][]string{f.Top, f.Bottom} {
		if len(list) < 1 || len(list) > 5 {
			return errors.New("top and bottom require 1 to 5 names each")
		}
		for _, name := range list {
			key := asciiLower(name)
			if strings.TrimSpace(name) == "" || seen[key] {
				return errors.New("empty or repeated officer name")
			}
			seen[key] = true
		}
	}
	return nil
}

// encoding/json repairs malformed UTF-8 and isolated UTF-16 surrogates. Names
// must not silently change. JSON syntax is checked before scanning escapes.
func validJSONText(data []byte) bool {
	if !utf8.Valid(data) || !json.Valid(data) {
		return false
	}
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if data[i] != 'u' {
			continue
		}
		value, _ := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		i += 4
		switch value >> 10 {
		case 54: // high surrogate: next escape must be a low surrogate
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return false
			}
			low, _ := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if low>>10 != 55 {
				return false
			}
			i += 6
		case 55:
			return false // isolated low surrogate
		}
	}
	return true
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
