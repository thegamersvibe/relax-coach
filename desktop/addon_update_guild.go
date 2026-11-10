package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Install scope is local policy, never supplied by the manifest.
var addonGuildRoots = []string{"Interface/AddOns/ForeverCoach", "Interface/AddOns/Journey", "Interface/Content/Journey-Content"}

// addonPlannerRoot joins the bundle from 1.5.0 (see addonBundleRelease).
const addonPlannerRoot = "Interface/AddOns/RelaxQuestPlanner"

func addonGuildRootsFor(version string) []string {
	if addonBundleRelease(version) {
		return append(append([]string{}, addonGuildRoots...), addonPlannerRoot)
	}
	return addonGuildRoots
}

var addonGuildExtras = map[string]bool{"RelaxCoach.exe": true, "README.md": true, "LICENSE-Coach": true, "Install-RelaxCoach.ps1": true, "Restore-RelaxCoach.ps1": true, "manifest.json": true}

func addonGuildRoot(name string, roots []string) string {
	for _, root := range roots {
		if strings.HasPrefix(name, root+"/") {
			return root
		}
	}
	return ""
}

func addonGuildPath(name string, directory bool, coachFiles map[string]bool, roots []string) bool {
	if directory {
		for _, root := range roots {
			if name == root || strings.HasPrefix(root, name+"/") || strings.HasPrefix(name, root+"/") {
				return true
			}
		}
		return false
	}
	root := addonGuildRoot(name, roots)
	switch root {
	case addonGuildRoots[0]:
		return coachFiles[strings.TrimPrefix(name, root+"/")]
	case addonGuildRoots[1], addonPlannerRoot:
		switch strings.ToLower(path.Ext(name)) {
		case ".lua", ".toc", ".tga", ".blp", ".xml", ".txt", ".md", ".ttf", ".otf", ".html", ".png":
			return true
		}
	case addonGuildRoots[2]:
		return path.Ext(name) == ".html"
	default:
		return addonGuildExtras[name]
	}
	return false
}

func (u *addonUpdater) extractGuild(ctx context.Context, payload []byte, stage, version string) (map[string]addonFileDigest, error) {
	coachFiles := addonPublicFilesForVersion(version)
	roots := addonGuildRootsFor(version)
	planner := len(roots) > len(addonGuildRoots)
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("invalid addon ZIP: %w", err)
	}
	if len(reader.File) > u.limits.entries {
		return nil, errors.New("addon archive exceeds the file-count limit")
	}
	seen := map[string]bool{}
	spellings := map[string]string{}
	directories := map[string]bool{}
	files := map[string]bool{}
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
		// ZIPs usually omit directories. Compare every implicit ancestor too, so
		// lib/a.lua and Lib/b.lua cannot merge on a case-insensitive filesystem.
		name := strings.TrimSuffix(f.Name, "/")
		for component := name; component != "."; component = path.Dir(component) {
			lower := strings.ToLower(component)
			if prior, ok := spellings[lower]; ok && prior != component {
				return nil, fmt.Errorf("case-colliding archive ancestor: %s", component)
			}
			spellings[lower] = component
			directory := component != name || f.Mode().IsDir()
			if (directory && files[lower]) || (!directory && directories[lower]) {
				return nil, fmt.Errorf("file/directory collision: %s", component)
			}
			if directory {
				directories[lower] = true
			} else {
				files[lower] = true
			}
		}
		if kind := f.Mode() & os.ModeType; kind != 0 && kind != os.ModeDir {
			return nil, fmt.Errorf("archive contains a symlink or special file: %s", f.Name)
		}
		if !addonGuildPath(strings.TrimSuffix(f.Name, "/"), f.Mode().IsDir(), coachFiles, roots) {
			return nil, fmt.Errorf("archive path is outside the fixed guild allowlist: %s", f.Name)
		}
		// One TOC per addon root: WoW would also load flavour TOCs such as
		// <Folder>_Vanilla.toc, which the identity checks below never read.
		if !f.Mode().IsDir() && strings.EqualFold(path.Ext(f.Name), ".toc") {
			for root, toc := range map[string]string{addonGuildRoots[1]: "Journey.toc", addonPlannerRoot: "RelaxQuestPlanner.toc"} {
				if strings.HasPrefix(f.Name, root+"/") && f.Name != root+"/"+toc {
					return nil, fmt.Errorf("archive TOC %s refused: only %s is allowed in %s", f.Name, toc, root)
				}
			}
		}
		if f.Mode().IsDir() {
			if f.UncompressedSize64 != 0 {
				return nil, errors.New("nonempty archive directory")
			}
			continue
		}
		if f.UncompressedSize64 > uint64(u.limits.file) || f.UncompressedSize64 > uint64(u.limits.expanded)-expanded {
			return nil, errors.New("addon archive exceeds the expanded size limit")
		}
		expanded += f.UncompressedSize64
	}
	for name := range coachFiles {
		if !files[strings.ToLower(addonGuildRoots[0]+"/"+name)] {
			return nil, fmt.Errorf("incomplete addon archive: missing %s", name)
		}
	}
	required := []string{addonGuildRoots[1] + "/Journey.toc", addonGuildRoots[2] + "/guides.html", "manifest.json"}
	if planner {
		required = append(required, addonPlannerRoot+"/RelaxQuestPlanner.toc")
	}
	for _, name := range required {
		if !seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("incomplete guild archive: missing %s", name)
		}
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		return nil, err
	}
	all := map[string]addonFileDigest{}
	runtime := map[string]addonFileDigest{}
	var manifest bytes.Buffer
	for _, f := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if f.Mode().IsDir() {
			continue
		}
		in, err := f.Open()
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		var out *os.File
		var dst io.Writer = h
		if addonGuildRoot(f.Name, roots) != "" {
			target := filepath.Join(stage, filepath.FromSlash(f.Name))
			if err = os.MkdirAll(filepath.Dir(target), 0700); err == nil {
				out, err = os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			}
			if err != nil {
				in.Close()
				return nil, err
			}
			dst = io.MultiWriter(out, h)
		} else if f.Name == "manifest.json" {
			if f.UncompressedSize64 > 1<<20 {
				in.Close()
				return nil, errors.New("guild manifest exceeds size limit")
			}
			dst = io.MultiWriter(&manifest, h)
		}
		n, copyErr := io.Copy(dst, io.LimitReader(addonContextReader{ctx, in}, u.limits.file+1))
		closeErr := in.Close()
		if out != nil {
			if copyErr == nil {
				copyErr = out.Sync()
			}
			closeErr = errors.Join(closeErr, out.Close())
		}
		if copyErr != nil || closeErr != nil {
			return nil, fmt.Errorf("cannot extract %s: %w", f.Name, errors.Join(copyErr, closeErr))
		}
		if n > u.limits.file || uint64(n) != f.UncompressedSize64 {
			return nil, errors.New("archive member exceeds declared size or extraction limit")
		}
		var sum [32]byte
		copy(sum[:], h.Sum(nil))
		digest := addonFileDigest{sum, n}
		if f.Name != "manifest.json" {
			all[f.Name] = digest
		}
		if out != nil {
			runtime[f.Name] = digest
		}
	}
	var m struct {
		Project        string `json:"project"`
		Kind           string `json:"kind"`
		Version        string `json:"version"`
		JourneyVersion string `json:"journeyVersion"`
		ContentVersion string `json:"contentVersion"`
		PlannerVersion string `json:"plannerVersion,omitempty"`
		Files          []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Bytes  int64  `json:"bytes"`
		} `json:"files"`
	}
	dec := json.NewDecoder(&manifest)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid guild manifest: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing guild manifest data")
	}
	if m.Project != "Relax Coach" || m.Kind != "guild-combined" || m.Version != version || m.JourneyVersion == "" || m.ContentVersion == "" || planner != (m.PlannerVersion != "") {
		return nil, errors.New("incompatible guild manifest identity/version")
	}
	if len(m.Files) != len(all) {
		return nil, errors.New("guild manifest inventory differs from archive")
	}
	declared := map[string]bool{}
	for _, row := range m.Files {
		expected, ok := all[row.Path]
		digest, err := hex.DecodeString(row.SHA256)
		if !ok || declared[row.Path] || err != nil || !bytes.Equal(digest, expected.sum[:]) || row.Bytes != expected.size {
			return nil, fmt.Errorf("guild manifest inventory/hash mismatch: %s", row.Path)
		}
		declared[row.Path] = true
	}
	toc, err := addonReadSmallFile(filepath.Join(stage, filepath.FromSlash(addonGuildRoots[0]), "ForeverCoach.toc"))
	if err != nil {
		return nil, err
	}
	if err := validateAddonTOC(toc, version); err != nil {
		return nil, err
	}
	toc, err = addonReadSmallFile(filepath.Join(stage, filepath.FromSlash(addonGuildRoots[1]), "Journey.toc"))
	if err != nil {
		return nil, err
	}
	meta, loads, err := addonTOCMetadata(toc)
	if err != nil {
		return nil, err
	}
	if meta["title"] != "Relax Journey" || meta["version"] != m.JourneyVersion || meta["savedvariables"] != "Journey_Account" || meta["savedvariablespercharacter"] != "Journey_Character" {
		return nil, errors.New("incompatible Journey TOC identity/version/SavedVariables")
	}
	for _, key := range []string{"dependencies", "requireddeps", "requireddependencies", "deps", "required-dependencies", "required-deps"} {
		for _, dep := range strings.FieldsFunc(meta[key], func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			if lower := strings.ToLower(dep); lower == "forevercoach" || strings.HasPrefix(lower, "forevercoach_") {
				return nil, errors.New("incompatible Journey TOC: Coach cannot be a required dependency")
			}
		}
	}
	for _, name := range loads {
		name = strings.ReplaceAll(name, "\\", "/")
		if err := addonSafeZipName(name); err != nil {
			return nil, err
		}
		if _, ok := runtime[addonGuildRoots[1]+"/"+name]; !ok {
			return nil, fmt.Errorf("missing Journey TOC load file: %s", name)
		}
	}
	if len(loads) == 0 || runtime[addonGuildRoots[2]+"/guides.html"].size == 0 {
		return nil, errors.New("incomplete Journey runtime or guides.html")
	}
	if planner {
		if err := addonCheckPlannerTOC(stage, runtime, m.PlannerVersion); err != nil {
			return nil, err
		}
	}
	return runtime, nil
}

// addonCheckPlannerTOC mirrors the Journey checks: fixed identity and
// SavedVariables, the manifest's version, no required Coach/Journey dependency
// and every load file present in the archive.
func addonCheckPlannerTOC(stage string, runtime map[string]addonFileDigest, version string) error {
	toc, err := addonReadSmallFile(filepath.Join(stage, filepath.FromSlash(addonPlannerRoot), "RelaxQuestPlanner.toc"))
	if err != nil {
		return err
	}
	meta, loads, err := addonTOCMetadata(toc)
	if err != nil {
		return err
	}
	if meta["title"] != "Relax Quest Planner" || meta["version"] != version || meta["savedvariablespercharacter"] != "RelaxQuestPlannerDB" || meta["savedvariables"] != "" {
		return errors.New("incompatible Relax Quest Planner TOC identity/version/SavedVariables")
	}
	for _, key := range []string{"dependencies", "requireddeps", "requireddependencies", "deps", "required-dependencies", "required-deps"} {
		if strings.TrimSpace(meta[key]) != "" {
			return errors.New("incompatible Relax Quest Planner TOC: it must not require other addons")
		}
	}
	for _, name := range loads {
		name = strings.ReplaceAll(name, "\\", "/")
		if err := addonSafeZipName(name); err != nil {
			return err
		}
		if _, ok := runtime[addonPlannerRoot+"/"+name]; !ok {
			return fmt.Errorf("missing Relax Quest Planner TOC load file: %s", name)
		}
	}
	if len(loads) == 0 {
		return errors.New("incomplete Relax Quest Planner runtime")
	}
	return nil
}
