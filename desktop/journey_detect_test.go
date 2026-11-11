//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// مانع round 2 (N5, N6) with Astra's TOC variants: the Title check reads
// what WoW reads, tolerantly, and an unreadable TOC is "unknown", never
// "replaced".
func TestJourneyTitleVariants(t *testing.T) {
	for _, c := range []struct{ name, file, toc, want string }{
		{"standard CRLF", "Journey.toc", "## Interface: 16001\r\n## Title: Relax Journey\r\n", "installed"},
		{"BOM before the Title line", "Journey.toc", "\xef\xbb\xbf## Title: Relax Journey\n", "installed"},
		{"bare CR lines", "Journey.toc", "## Interface: 16001\r## Title: Relax Journey\r", "installed"},
		{"color codes around our title", "Journey.toc", "## Title: |cff33ff99Relax Journey|r\n", "installed"},
		{"same Title twice", "Journey.toc", "## Title: Relax Journey\n## Title: Relax Journey\n", "installed"},
		{"other metadata twice", "Journey.toc", "## Title: Relax Journey\n## Notes: one\n## Notes: two\n", "installed"},
		{"lowercase key, spaces", "Journey.toc", "##  title : Relax Journey \t\n", "installed"},
		{"lowercase file name", "journey.toc", "## Title: Relax Journey\n", "installed"},
		{"localized Title beside ours", "Journey.toc", "## Title: Relax Journey\n## Title-deDE: Entspannte Reise\n", "installed"},
		{"over 64 KiB", "Journey.toc", "## Title: Relax Journey\n#" + strings.Repeat("x", 70000) + "\n", "installed"},
		{"normal Journey", "Journey.toc", "## Title: Journey\r\n", "replaced"},
		{"foreign Title first", "Journey.toc", "## Title: Journey\n## Title: Relax Journey\n", "replaced"},
		{"foreign Title last", "Journey.toc", "## Title: Relax Journey\n## Title: Journey\n", "replaced"},
		{"localized Title only", "Journey.toc", "## Title-deDE: Relax Journey\n", "replaced"},
		{"empty TOC", "Journey.toc", "", "replaced"},
		{"only a flavour TOC", "Journey_Vanilla.toc", "## Title: Relax Journey\n", "replaced"},
	} {
		t.Run(c.name, func(t *testing.T) {
			game := installedBundle(t, "1.5.2")
			os.RemoveAll(journeyFolder(game))
			addonPut(t, game, "Interface/AddOns/Journey/"+c.file, c.toc)
			addonPut(t, game, "Interface/AddOns/Journey/Main.xml", "<Ui/>")
			if s := InstalledBundleState(game); s.State != c.want || s.JourneyUnreadable {
				t.Fatalf("got %+v, want %s", s, c.want)
			}
		})
	}
}

func TestJourneyUnreadableTOCIsUnknown(t *testing.T) {
	game := installedBundle(t, "1.5.2")
	toc := filepath.Join(journeyFolder(game), "Journey.toc")
	if err := os.Chmod(toc, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(toc, 0644)
	if _, err := os.ReadFile(toc); err == nil {
		t.Skip("running with permissions that can read a 0000 file")
	}
	s := InstalledBundleState(game)
	if s.State == "replaced" || !s.JourneyUnreadable {
		t.Fatalf("unreadable TOC: %+v, want not replaced and journeyUnreadable", s)
	}
	if addonRepairNeeded(game) {
		t.Fatal("an unreadable TOC must not offer Repair")
	}
}

// N7: the docs that promise Repair also say it needs a published release,
// and none claims the folder was certainly replaced by another updater.
func TestJourneyRepairDocsNeedARelease(t *testing.T) {
	for _, doc := range []string{"../README.md", "../docs/PUBLIC_README.md"} {
		b, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if !strings.Contains(text, "it needs a published Relax Coach release") || strings.Contains(text, "Journey was replaced by another updater") {
			t.Fatalf("%s: Repair promise without the release condition, or a certain-replacement claim", doc)
		}
	}
}
