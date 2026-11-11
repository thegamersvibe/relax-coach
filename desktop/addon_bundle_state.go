package main

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// AddonBundleState describes which Relax Coach bundle folders exist in a game
// folder. It is read-only and offline; it only decides wording (Install or
// Update) and explains a partial install. Installing always goes through the
// same verified updater transaction.
//
//   - "none": none of the bundle folders exist. The action is Install.
//   - "installed": ForeverCoach has a readable version and every folder that
//     bundle version ships is present. The action is Update.
//   - "replaced": Coach is installed but the Journey folder is not Relax
//     Journey (no Journey.toc, or its Title is not "Relax Journey"). Another
//     updater (CurseForge, Wago, Journey Guide Manager) may have replaced it,
//     or the player keeps the normal Journey on purpose. Repair is offered
//     through the same transaction; the other copy goes to the backup.
//   - "partial": anything else, e.g. only a normal Journey, or Coach without
//     Journey-Content. The action is Install when Coach itself is missing or
//     unreadable, otherwise Update; Missing lists what the next install or
//     update puts back.
//
// A RelaxQuestPlanner folder is only expected from bundle 1.5.0: a 1.4.x
// install without it is "installed" (the next update adds it).
type AddonBundleState struct {
	State          string   `json:"state"`
	Install        bool     `json:"install"`
	PresentFolders []string `json:"present"`
	MissingFolders []string `json:"missing"`
	// JourneyUnreadable: Journey.toc could not be read, so it is unknown
	// whether the Journey folder is Relax Journey (no Repair is offered).
	JourneyUnreadable bool `json:"journeyUnreadable,omitempty"`
}

func InstalledBundleState(game string) AddonBundleState {
	all := addonGuildRootsFor(addonBundleMinimumVersion)
	state := AddonBundleState{State: "none", Install: true, PresentFolders: []string{}, MissingFolders: []string{}}
	if game == "" || !filepath.IsAbs(game) {
		state.MissingFolders = addonFolderNames(all)
		return state
	}
	game = filepath.Clean(game)
	present := map[string]bool{}
	for _, root := range all {
		// Only a real, non-empty folder counts, reached without links (a file, a
		// link, an empty folder or a linked Interface\Content is not an install).
		dir := filepath.Join(game, filepath.FromSlash(root))
		if addonPlainDirectory(dir) == nil {
			if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
				present[root] = true
			}
		}
	}
	version := InstalledCoachVersion(game)
	state.Install = version == "0.0.0"
	expected := all
	if !state.Install {
		expected = []string{addonGuildRoots[0]}
		if addonGuildRelease(version) {
			expected = addonGuildRootsFor(version)
		}
	}
	for _, root := range all {
		if present[root] {
			state.PresentFolders = append(state.PresentFolders, path.Base(root))
		}
	}
	for _, root := range expected {
		if !present[root] {
			state.MissingFolders = append(state.MissingFolders, path.Base(root))
		}
	}
	journeyForeign := false
	if !state.Install && present[addonGuildRoots[1]] {
		ours, readable := addonJourneyTitle(filepath.Join(game, filepath.FromSlash(addonGuildRoots[1])))
		journeyForeign = readable && !ours
		state.JourneyUnreadable = !readable
	}
	switch {
	case len(present) == 0:
		state.State = "none"
	case !state.Install && present[addonGuildRoots[1]] && journeyForeign:
		state.State = "replaced"
	case !state.Install && len(state.MissingFolders) == 0:
		state.State = "installed"
	default:
		state.State = "partial"
	}
	return state
}

// addonJourneyTitle reads the folder's Journey.toc (any case, as on Windows)
// the way WoW reads metadata, tolerantly: a UTF-8 BOM, CRLF/LF/CR line ends,
// any spacing and key case around "## Title:", and WoW color codes are
// accepted. Every base Title line must say "Relax Journey" (localized
// Title-xx lines are ignored). ours=false with readable=true means another
// Journey (or no TOC / no Title); readable=false means the TOC could not be
// read (permission, I/O, oversized), which is unknown, not foreign.
func addonJourneyTitle(dir string) (ours, readable bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, false
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), "Journey.toc") {
			continue
		}
		if !entry.Type().IsRegular() {
			return false, true
		}
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			return false, false
		}
		defer f.Close()
		toc, err := io.ReadAll(io.LimitReader(f, addonJourneyTOCLimit+1))
		if err != nil || len(toc) > addonJourneyTOCLimit {
			return false, false
		}
		return addonTOCTitleIsRelax(toc), true
	}
	return false, true
}

const addonJourneyTOCLimit = 1 << 20

var addonColorCode = regexp.MustCompile(`\|c[0-9A-Fa-f]{8}|\|r`)

func addonTOCTitleIsRelax(toc []byte) bool {
	text := strings.TrimPrefix(string(toc), "\ufeff")
	titles := 0
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "##") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "##"), ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Title") {
			continue
		}
		if strings.TrimSpace(addonColorCode.ReplaceAllString(value, "")) != "Relax Journey" {
			return false
		}
		titles++
	}
	return titles > 0
}

func addonFolderNames(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		out = append(out, path.Base(root))
	}
	return out
}
