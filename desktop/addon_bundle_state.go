package main

import (
	"os"
	"path"
	"path/filepath"
)

// AddonBundleState describes which Relax Coach bundle folders exist in a game
// folder. It is read-only and offline; it only decides wording (Install or
// Update) and explains a partial install. Installing always goes through the
// same verified updater transaction.
//
//   - "none": none of the bundle folders exist. The action is Install.
//   - "installed": ForeverCoach has a readable version and every folder that
//     bundle version ships is present. The action is Update.
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
		// Only a real folder counts; a file or a link there is not an install.
		if st, err := os.Lstat(filepath.Join(game, filepath.FromSlash(root))); err == nil && st.IsDir() {
			present[root] = true
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
	case !state.Install && len(state.MissingFolders) == 0:
		state.State = "installed"
	default:
		state.State = "partial"
	}
	return state
}

func addonFolderNames(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		out = append(out, path.Base(root))
	}
	return out
}
