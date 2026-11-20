package main

// Optional name sources for Farming. All files are data, never Lua execution.
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const coachNameMaxItems = 10000

var (
	gearNameMaxBytes      int64 = 4 << 20 // per file, like AH Profit's reference reader
	gearNameMaxTotalBytes int64 = 32 << 20
	gearNameMaxFiles            = 64
	gearNameMaxValues           = 250000 // per table, including nested values
	gearNameMaxItems            = 50000  // across every installed class
)
var gearClasses = []string{"DRUID", "HUNTER", "MAGE", "PALADIN", "PRIEST", "ROGUE", "SHAMAN", "WARLOCK", "WARRIOR"}

// Both the shared audit and the class item facts carry name/quality fields.
// Match assignments only; everything outside the literal is ignored, not run.
var gearNameAssignment = regexp.MustCompile(`(?m)^((?:GQ|GearQuest)\.Data\.(?:foreverAudit|itemFacts|(?:druid|hunter|mage|paladin|priest|rogue|shaman|warlock|warrior)(?:ItemFacts|Early1to9Facts)))\s*=`)

type gearNameCache struct {
	key   string
	items map[int]farmItemRef
}

func farmNameID(k string) (int, bool) {
	id, err := strconv.Atoi(k)
	return id, err == nil && id > 0 && id <= 10000000 && strconv.Itoa(id) == k
}
func validFarmName(name string) bool {
	if name == "" || len(name) > 128 || !utf8.ValidString(name) || strings.TrimSpace(name) == "" || strings.ContainsRune(name, '|') {
		return false
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
func farmQualityClass(v any) string {
	q, ok := ahpInt(v, 8)
	if !ok || q == 8 {
		return ""
	}
	return "q" + strconv.Itoa(q)
}
func coachItemNames(db map[string]any) map[int]farmItemRef {
	rows := M(db["itemNames"])
	// The addon never writes more. Refuse an oversized map rather than pick
	// arbitrary entries from Go's randomized map iteration order.
	if len(rows) > coachNameMaxItems {
		return nil
	}
	out := map[int]farmItemRef{}
	for k, raw := range rows {
		id, ok := farmNameID(k)
		r := M(raw)
		name, _ := r["n"].(string)
		if !validFarmName(name) {
			name = ""
		}
		quality := farmQualityClass(r["q"])
		if ok && (name != "" || quality != "") {
			out[id] = farmItemRef{Name: name, QualityClass: quality}
		}
	}
	return out
}
func fillFarmNames(into, names map[int]farmItemRef) {
	for id, src := range names {
		dst := into[id]
		if dst.Name == "" && src.Name != "" {
			dst.Name = src.Name
		}
		// Quality is an ID fact, independent of the selected name text.
		if dst.QualityClass == "" {
			dst.QualityClass = src.QualityClass
		}
		into[id] = dst
	}
}

// farmNameProgress measures saved name coverage, not the client's queue or
// resolvability. A missing merged name implies Coach is absent/incomplete too.
func farmNameProgress(db map[string]any, items map[int]farmItemRef) string {
	seen := map[int]bool{}
	named := 0
	for _, raw := range M(db["hist"]) {
		for key, value := range M(raw) {
			id, ok := farmNameID(key)
			_, packed := value.(string)
			if !ok || !packed || seen[id] {
				continue
			}
			seen[id] = true
			if items[id].Name != "" {
				named++
			}
		}
	}
	if named < len(seen) {
		return fmt.Sprintf("%d of %d item names known", named, len(seen))
	}
	return ""
}

// readGearFiles visits only the known addon roots and their _generated folder.
// Every directory and file must be plain (no symlinks). Directory enumeration,
// file bytes and total bytes are bounded before parsing any untrusted data.
func readGearFiles(game string) (sources []string, key string) {
	root, err := os.OpenRoot(game)
	if err != nil {
		return nil, ""
	}
	defer root.Close()
	inter, err := openPlainDirectory(root, "Interface")
	if err != nil {
		return nil, ""
	}
	defer inter.Close()
	addons, err := openPlainDirectory(inter, "AddOns")
	if err != nil {
		return nil, ""
	}
	defer addons.Close()
	folders := []string{"GearQuestForever"}
	for _, class := range gearClasses {
		folders = append(folders, "GearQuestForever_"+class)
	}
	var total int64
	var files int
	var keys []string
	for _, folder := range folders {
		dir, err := openPlainDirectory(addons, folder)
		if err != nil {
			continue
		}
		for _, sub := range []string{"", "_generated"} {
			d := dir
			if sub != "" {
				d, err = openPlainDirectory(dir, sub)
				if err != nil {
					continue
				}
			}
			f, e := d.Open(".")
			var entries []os.DirEntry
			if e == nil {
				entries, e = f.ReadDir(gearNameMaxFiles + 1)
				f.Close()
			}
			if e != nil && e != io.EOF || len(entries) > gearNameMaxFiles {
				if d != dir {
					d.Close()
				}
				continue
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			for _, entry := range entries {
				name := entry.Name()
				if !strings.HasSuffix(name, ".generated.lua") {
					continue
				}
				files++
				if files > gearNameMaxFiles {
					if d != dir {
						d.Close()
					}
					dir.Close()
					return nil, ""
				}
				st, e := d.Lstat(name)
				if e != nil || !st.Mode().IsRegular() || st.Size() > gearNameMaxBytes {
					continue
				}
				total += st.Size()
				if total > gearNameMaxTotalBytes {
					if d != dir {
						d.Close()
					}
					dir.Close()
					return nil, ""
				}
				f, e := d.Open(name)
				if e != nil {
					continue
				}
				opened, e := f.Stat()
				if e != nil || !os.SameFile(st, opened) {
					f.Close()
					continue
				}
				b, e := io.ReadAll(io.LimitReader(f, gearNameMaxBytes+1))
				f.Close()
				if e != nil || int64(len(b)) > gearNameMaxBytes || int64(len(b)) != st.Size() {
					continue
				}
				sources = append(sources, string(b))
				keys = append(keys, fmt.Sprintf("%s/%s/%s:%d:%d", folder, sub, name, st.Size(), st.ModTime().UnixNano()))
			}
			if d != dir {
				d.Close()
			}
		}
		dir.Close()
	}
	return sources, filepath.Clean(game) + "\x00" + strings.Join(keys, "|")
}

// gearNameSource masks comments for the literal parser, and strings as well
// for assignment discovery. Keep offsets/newlines identical; parse only from
// the discovered assignment, never a preceding copy inside inert text.
func gearNameSource(src string) (string, string) {
	data, code := []byte(src), []byte(src)
	mask := func(b []byte, start, end int) {
		for i := start; i < end; i++ {
			if b[i] != '\n' && b[i] != '\r' {
				b[i] = ' '
			}
		}
	}
	longEnd := func(start int) (int, bool) {
		if start >= len(src) || src[start] != '[' {
			return start, false
		}
		j := start + 1
		for j < len(src) && src[j] == '=' {
			j++
		}
		if j >= len(src) || src[j] != '[' {
			return start, false
		}
		close := "]" + src[start+1:j] + "]"
		if end := strings.Index(src[j+1:], close); end >= 0 {
			return j + 1 + end + len(close), true
		}
		return len(src), true // unterminated inert text cannot expose code
	}
	for i := 0; i < len(src); {
		start := i
		switch {
		case strings.HasPrefix(src[i:], "--"):
			var long bool
			i, long = longEnd(i + 2)
			if !long {
				for i < len(src) && src[i] != '\n' && src[i] != '\r' {
					i++
				}
			}
			mask(data, start, i)
		case src[i] == '\'' || src[i] == '"':
			quote := src[i]
			i++
			for i < len(src) {
				c := src[i]
				i++
				if c == '\\' && i < len(src) {
					i++
				} else if c == quote {
					break
				}
			}
		case src[i] == '[':
			end, long := longEnd(i)
			if !long {
				i++
				continue
			}
			i = end
		default:
			i++
			continue
		}
		mask(code, start, i)
	}
	return string(data), string(code)
}

func loadGearNames(game string, cache *gearNameCache) map[int]farmItemRef {
	if game == "" {
		return nil
	}
	sources, key := readGearFiles(game)
	if cache != nil && key != "" && cache.key == key {
		return cache.items
	}
	out := map[int]farmItemRef{}
	for _, src := range sources {
		data, code := gearNameSource(src)
		matches := gearNameAssignment.FindAllStringSubmatchIndex(code, 17)
		if len(matches) > 16 {
			continue
		} // a generated file normally has one name table
		seen := map[string]bool{}
		for _, match := range matches {
			name := code[match[2]:match[3]]
			if seen[name] {
				continue
			}
			seen[name] = true
			d, err := parseLuaAssignedTableLimited(data[match[0]:], name, gearNameMaxValues)
			if err != nil {
				continue
			}
			if len(d) > gearNameMaxItems {
				continue
			}
			for k, raw := range d {
				id, ok := farmNameID(k)
				r := M(raw)
				name, _ := r["name"].(string)
				if !validFarmName(name) {
					name = ""
				}
				quality := farmQualityClass(r["quality"])
				if !ok || (name == "" && quality == "") {
					continue
				}
				if _, have := out[id]; !have {
					if len(out) >= gearNameMaxItems {
						return nil
					} // reject, never a random partial map
					out[id] = farmItemRef{Name: name, QualityClass: quality}
				} else {
					// First known value wins per field, as across providers:
					// a quality-only row must not hide a later usable name.
					item := out[id]
					if item.Name == "" {
						item.Name = name
					}
					if item.QualityClass == "" {
						item.QualityClass = quality
					}
					out[id] = item
				}
			}
		}
	}
	if cache != nil {
		*cache = gearNameCache{key: key, items: out}
	}
	return out
}
