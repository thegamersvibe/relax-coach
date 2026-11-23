package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

var retirementRunName = regexp.MustCompile(`^\d{8}T\d{6}Z(-[1-9]\d*)?$`)

func (q *retirementQuarantine) clearPending() error {
	// Fixed internal journal name only, never source data or quarantined data.
	err := q.root.Remove(filepath.Join(q.run, "pending.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (q *retirementQuarantine) readManifest(name string) (retirementManifest, error) {
	var doc retirementManifest
	path := filepath.Join(q.run, name)
	st, err := q.root.Lstat(path)
	if err != nil {
		return doc, err
	}
	if !st.Mode().IsRegular() {
		return doc, fmt.Errorf("non-regular retirement journal")
	}
	if err = appRejectReparse(filepath.Join(q.game, path)); err != nil {
		return doc, err
	}
	data, err := q.root.ReadFile(path)
	if err != nil {
		return doc, err
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if doc.Schema != 1 {
		return doc, fmt.Errorf("unknown retirement journal schema")
	}
	seen := map[string]bool{}
	for _, e := range doc.Entries {
		if !q.validEntry(e) || seen[e.Source] {
			return doc, fmt.Errorf("invalid retirement journal entry")
		}
		seen[e.Source] = true
	}
	return doc, nil
}
func (q *retirementQuarantine) validEntry(e retirementEntry) bool {
	source := filepath.FromSlash(e.Source)
	dest := filepath.FromSlash(e.Destination)
	if !filepath.IsLocal(source) || filepath.ToSlash(filepath.Clean(source)) != e.Source {
		return false
	}
	parts := strings.Split(e.Source, "/")
	mirror := source
	if len(parts) >= 3 && parts[0] == "Interface" && parts[1] == "AddOns" {
		switch parts[2] {
		case "ForeverCoach", "RelaxRaid", "RelaxQuestPlanner", "Journey":
		default:
			return false
		}
		mirror = filepath.Join(parts[1:]...)
	} else {
		if (len(parts) != 5 && len(parts) != 7) || parts[0] != "WTF" || parts[1] != "Account" || parts[len(parts)-2] != "SavedVariables" || e.Type != "file" {
			return false
		}
		name := parts[len(parts)-1]
		match := false
		for _, stem := range append(append([]string{}, retirementSaveNames...), retirementJourneySaveNames...) {
			if name == stem+".lua" || name == stem+".lua.bak" {
				match = true
			}
		}
		if !match {
			return false
		}
	}
	if dest != filepath.Join(q.run, mirror) || filepath.ToSlash(dest) != e.Destination {
		return false
	}
	if e.Type == "dir" {
		return e.SHA256 == ""
	}
	return e.Type == "file" && len(e.SHA256) == 64 && strings.Trim(e.SHA256, "0123456789abcdef") == ""
}
func (q *retirementQuarantine) verify(entries []retirementEntry) error {
	top := entries[0]
	target := filepath.FromSlash(top.Destination)
	got, err := q.inventory(target, target)
	if err != nil {
		return err
	}
	for i := range got {
		got[i].Source = top.Source + strings.TrimPrefix(got[i].Source, top.Destination)
	}
	if !reflect.DeepEqual(got, entries) {
		return fmt.Errorf("moved contents differ from the pre-move inventory")
	}
	return nil
}
func (q *retirementQuarantine) recover() error {
	dirs, err := os.ReadDir(filepath.Join(q.game, retirementStorage))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, d := range dirs {
		if !retirementRunName.MatchString(d.Name()) {
			continue
		}
		run := filepath.Join(retirementStorage, d.Name())
		if err = appPlainDirectory(filepath.Join(q.game, run)); err != nil {
			return err
		}
		prior := &retirementQuarantine{game: q.game, root: q.root, run: run}
		intent, err := prior.readManifest("pending.json")
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if len(intent.Entries) == 0 {
			return fmt.Errorf("empty retirement intent")
		}
		top := intent.Entries[0]
		_, sourceErr := q.root.Lstat(filepath.FromSlash(top.Source))
		_, destErr := q.root.Lstat(filepath.FromSlash(top.Destination))
		if sourceErr == nil && os.IsNotExist(destErr) {
			// Rename never happened. Forget only the intent and empty scaffolding;
			// the ordinary exact-scope pass can retry the still-present source.
			if err = prior.clearPending(); err != nil {
				return err
			}
			for dir := filepath.Dir(filepath.FromSlash(top.Destination)); dir != retirementStorage; dir = filepath.Dir(dir) {
				if err = prior.root.Remove(dir); err != nil {
					break
				}
			}
			continue
		}
		if !os.IsNotExist(sourceErr) || destErr != nil {
			return fmt.Errorf("ambiguous retirement journal in %s; kept for manual inspection", run)
		}
		// Completion is determined from the filesystem, not from an uncommitted
		// intent. Bound every path, reject links, and verify the full moved tree.
		target := filepath.FromSlash(top.Destination)
		if err = appPlainDirectory(filepath.Join(q.game, filepath.Dir(target))); err != nil {
			return err
		}
		previous, err := prior.readManifest("manifest.json")
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		prior.entries = previous.Entries
		for _, e := range intent.Entries {
			exists := false
			for _, p := range prior.entries {
				if p.Source == e.Source {
					if p != e {
						return fmt.Errorf("conflicting retirement journal")
					}
					exists = true
				}
			}
			if !exists {
				prior.entries = append(prior.entries, e)
			}
		}
		if err = prior.writeManifest(); err != nil {
			return err
		}
		if err = prior.verify(intent.Entries); err != nil {
			return fmt.Errorf("hash verification failed in %s (kept in quarantine)", run)
		}
		if err = prior.clearPending(); err != nil {
			return err
		}
	}
	return nil
}
