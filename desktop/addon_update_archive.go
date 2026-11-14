package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Deliberately authoritative and fail-closed. Adding an addon module requires a
// reviewed updater change, not a remote manifest granting itself more scope.
// All these files are required. Optional root files are validated/read but NEVER
// installed: RelaxCoach.exe, README.md, LICENSE, manifest.json.
var addonPublicFiles = map[string]bool{
	"ForeverCoach.toc": true, "Core.lua": true, "Rules.lua": true,
	"Route.lua": true, "Session.lua": true, "Gear.lua": true,
	"Talents.lua": true, "Dungeons.lua": true, "Audit.lua": true,
	"Macros.lua": true, "UI.lua": true, "ForeverCoach.tga": true,
}

// Preserve the legacy contract; only 1.4.3+ requires the reviewed Fun Mode files.
func addonPublicFilesForVersion(version string) map[string]bool {
	v, err := parseAddonVersion(version)
	minimum, _ := parseAddonVersion("1.4.3")
	if err != nil || minimum.newerThan(v) {
		return addonPublicFiles
	}
	files := make(map[string]bool, len(addonPublicFiles)+8)
	for name, allowed := range addonPublicFiles {
		files[name] = allowed
	}
	for _, name := range []string{
		"Reactions.lua",
		"Media/Reactions/bags_1.mp3", "Media/Reactions/bags_2.mp3",
		"Media/Reactions/death.mp3",
		"Media/Reactions/level_up.mp3", "Media/Reactions/level_up.tga",
	} {
		files[name] = true
	}
	// 1.5.6 replaced the one campfire song with two that take turns. Each bundle
	// version requires exactly its own set, so neither name is allowed in the other.
	songs, _ := parseAddonVersion("1.5.6")
	if songs.newerThan(v) {
		files["Media/Reactions/campfire.mp3"] = true
	} else {
		files["Media/Reactions/campfire_1.mp3"] = true
		files["Media/Reactions/campfire_2.mp3"] = true
	}
	return files
}

type addonFileDigest struct {
	sum  [32]byte
	size int64
}

type addonContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r addonContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func addonSafeZipName(name string) error {
	if name == "" || len(name) > 200 || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("unsafe archive path %q", name)
	}
	name = strings.TrimSuffix(name, "/")
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return fmt.Errorf("unsafe archive path %q", name)
		}
		for _, c := range part {
			if c < 32 || c > 126 || strings.ContainsRune(`<>"|?*`, c) {
				return fmt.Errorf("unsafe archive filename %q", name)
			}
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return fmt.Errorf("reserved Windows archive filename %q", name)
		}
	}
	return nil
}

func (u *addonUpdater) extract(ctx context.Context, payload []byte, stage, version string) (map[string]addonFileDigest, error) {
	if addonGuildRelease(version) {
		return u.extractGuild(ctx, payload, stage, version)
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("invalid addon ZIP: %w", err)
	}
	if len(reader.File) > u.limits.entries {
		return nil, errors.New("addon archive exceeds the file-count limit")
	}
	seen := map[string]bool{}
	var expanded uint64
	for _, f := range reader.File {
		if err := addonSafeZipName(f.Name); err != nil {
			return nil, err
		}
		key := strings.ToLower(strings.TrimSuffix(f.Name, "/"))
		if seen[key] {
			return nil, fmt.Errorf("duplicate or case-colliding archive path %q", f.Name)
		}
		seen[key] = true
		if kind := f.Mode() & os.ModeType; kind != 0 && kind != os.ModeDir {
			return nil, fmt.Errorf("archive contains a symlink or special file: %s", f.Name)
		}
		if f.Mode().IsDir() {
			if f.Name != "ForeverCoach/" || f.UncompressedSize64 != 0 {
				return nil, fmt.Errorf("unexpected archive directory %q", f.Name)
			}
			continue
		}
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("archive contains a non-regular file: %s", f.Name)
		}
		if strings.HasPrefix(f.Name, "ForeverCoach/") {
			if !addonPublicFiles[strings.TrimPrefix(f.Name, "ForeverCoach/")] {
				return nil, fmt.Errorf("archive file is outside the public addon allowlist: %s", f.Name)
			}
		} else {
			switch f.Name {
			case "RelaxCoach.exe", "README.md", "LICENSE", "manifest.json":
			default:
				return nil, fmt.Errorf("archive contains material outside ForeverCoach: %s", f.Name)
			}
		}
		if f.UncompressedSize64 > uint64(u.limits.file) || f.UncompressedSize64 > uint64(u.limits.expanded)-expanded {
			return nil, errors.New("addon archive exceeds the expanded size limit")
		}
		expanded += f.UncompressedSize64
	}
	for name := range addonPublicFiles {
		if !seen[strings.ToLower("ForeverCoach/"+name)] {
			return nil, fmt.Errorf("incomplete addon archive: missing ForeverCoach/%s", name)
		}
	}
	if err := os.Mkdir(stage, 0o700); err != nil {
		return nil, err
	}
	digests := map[string]addonFileDigest{}
	for _, f := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if f.Mode().IsDir() {
			continue
		}
		in, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("cannot open archive member %s: %w", f.Name, err)
		}
		var out *os.File
		var dst io.Writer = io.Discard
		name := strings.TrimPrefix(f.Name, "ForeverCoach/")
		hash := sha256.New()
		if strings.HasPrefix(f.Name, "ForeverCoach/") {
			out, err = os.OpenFile(filepath.Join(stage, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				in.Close()
				return nil, err
			}
			dst = io.MultiWriter(out, hash)
		}
		n, copyErr := io.Copy(dst, io.LimitReader(addonContextReader{ctx, in}, u.limits.file+1))
		closeErr := in.Close()
		if out != nil {
			if copyErr == nil {
				copyErr = out.Sync()
			}
			if err := out.Close(); copyErr == nil {
				copyErr = err
			}
		}
		if copyErr != nil {
			return nil, fmt.Errorf("cannot extract %s: %w", f.Name, copyErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if n > u.limits.file || uint64(n) != f.UncompressedSize64 {
			return nil, errors.New("archive member exceeds its declared size or extraction limit")
		}
		if out != nil {
			var sum [32]byte
			copy(sum[:], hash.Sum(nil))
			digests[name] = addonFileDigest{sum, n}
		}
	}
	toc, err := addonReadSmallFile(filepath.Join(stage, "ForeverCoach.toc"))
	if err != nil {
		return nil, err
	}
	if err := validateAddonTOC(toc, version); err != nil {
		return nil, err
	}
	return digests, nil
}

func addonReadSmallFile(p string) ([]byte, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, errors.New("addon TOC is not a small regular file")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if len(b) > 64<<10 {
		return nil, errors.New("addon TOC exceeds the size limit")
	}
	return b, err
}

func addonTOCMetadata(b []byte) (map[string]string, []string, error) {
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return nil, nil, errors.New("invalid addon TOC text")
	}
	meta := map[string]string{}
	var files []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "##") {
			key, value, found := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "##")), ":")
			if !found {
				continue
			}
			key = strings.ToLower(strings.TrimSpace(key))
			if _, exists := meta[key]; exists {
				return nil, nil, fmt.Errorf("duplicate addon TOC metadata: %s", key)
			}
			meta[key] = strings.TrimSpace(value)
		} else if line != "" && !strings.HasPrefix(line, "#") {
			files = append(files, line)
		}
	}
	return meta, files, nil
}

func validateAddonTOC(b []byte, version string) error {
	meta, files, err := addonTOCMetadata(b)
	if err != nil {
		return err
	}
	if meta["title"] != "Relax Coach" {
		return errors.New("incompatible addon package: TOC Title must be Relax Coach")
	}
	declaredVersion, versionErr := addonExactTOCVersion(b)
	if versionErr != nil || declaredVersion.text != version || meta["version"] != version {
		return errors.New("incompatible addon package: exact TOC Version field must match the release")
	}
	if meta["savedvariables"] != "ForeverCoachDB" || meta["savedvariablespercharacter"] != "" {
		return errors.New("incompatible addon package: SavedVariables must remain ForeverCoachDB")
	}
	for _, key := range []string{"dependencies", "requireddeps", "requireddependencies", "deps", "required-dependencies", "required-deps"} {
		for _, dep := range strings.FieldsFunc(meta[key], func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			if lower := strings.ToLower(dep); lower == "journey" || strings.HasPrefix(lower, "journey_") {
				return errors.New("incompatible addon package: Journey cannot be a required dependency")
			}
		}
	}
	publicFiles := addonPublicFilesForVersion(version)
	loaded := map[string]bool{}
	for _, name := range files {
		if !publicFiles[name] || !strings.HasSuffix(name, ".lua") || loaded[name] {
			return fmt.Errorf("unsafe or duplicate TOC load path %q", name)
		}
		loaded[name] = true
	}
	for name := range publicFiles {
		if strings.HasSuffix(name, ".lua") && !loaded[name] {
			return fmt.Errorf("incomplete addon TOC: missing module %s", name)
		}
	}
	return nil
}

func verifyAddonTree(ctx context.Context, root string, want map[string]addonFileDigest) error {
	if err := addonPlainDirectory(root); err != nil {
		return err
	}
	directories := map[string]bool{".": true}
	for name := range want {
		for parent := filepath.Dir(filepath.FromSlash(name)); parent != "."; parent = filepath.Dir(parent) {
			directories[filepath.ToSlash(parent)] = true
		}
	}
	count := 0
	err := filepath.WalkDir(root, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !directories[name] {
				return fmt.Errorf("addon verification found unexpected directory: %s", name)
			}
			return addonPlainDirectory(p)
		}
		expected, ok := want[name]
		if !ok || !info.Mode().IsRegular() || info.Size() != expected.size {
			return fmt.Errorf("addon verification failed for %s", name)
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(addonContextReader{ctx, f}, expected.size+1))
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr)
		}
		if n != expected.size || !bytes.Equal(h.Sum(nil), expected.sum[:]) {
			return fmt.Errorf("addon verification failed: SHA256 mismatch for %s", name)
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	if count != len(want) {
		return errors.New("addon verification failed: installed file count differs")
	}
	return ctx.Err()
}
