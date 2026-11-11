package main

import (
	"os"
	"path"
	"path/filepath"
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
//     Journey (its Journey.toc Title is not "Relax Journey"): another updater
//     (CurseForge, Wago, Journey Guide Manager) overwrote it. The action is
//     Repair, through the same transaction; the other copy goes to the backup.
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
	switch {
	case len(present) == 0:
		state.State = "none"
	case !state.Install && present[addonGuildRoots[1]] && !addonJourneyIsRelax(filepath.Join(game, filepath.FromSlash(addonGuildRoots[1]))):
		state.State = "replaced"
	case !state.Install && len(state.MissingFolders) == 0:
		state.State = "installed"
	default:
		state.State = "partial"
	}
	return state
}

// addonJourneyIsRelax: the folder's Journey.toc (any case, as on Windows) has
// Title "Relax Journey". A missing or unreadable TOC is not ours.
func addonJourneyIsRelax(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), "Journey.toc") || !entry.Type().IsRegular() {
			continue
		}
		toc, err := addonReadSmallFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return false
		}
		meta, _, err := addonTOCMetadata(toc)
		return err == nil && meta["title"] == "Relax Journey"
	}
	return false
}

func addonFolderNames(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		out = append(out, path.Base(root))
	}
	return out
}
