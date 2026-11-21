package main

import (
	"bytes"
	"errors"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestRaidPageReadStream(t *testing.T) {
	var raw bytes.Buffer
	at := time.Date(2026, 12, 9, 20, 0, 0, 0, time.UTC)
	fixture.Write(&raw, at, 3, "ENCOUNTER_START", fixture.Number(42), fixture.Text("Training Colossus"), fixture.Number(1), fixture.Number(10), fixture.Number(99))
	fixture.Write(&raw, at.Add(time.Minute), 3, "ENCOUNTER_END", fixture.Number(42), fixture.Text("Training Colossus"), fixture.Number(1), fixture.Number(10), fixture.Number(1), fixture.Number(60000))
	pages, err := readRaidPages(bytes.NewReader(raw.Bytes()), nil, raidPrepConfig(t))
	if err != nil || len(pages) != 1 || pages[0].Bosses != 1 {
		t.Fatalf("stream not rendered: %v", err)
	}
	html, err := renderRaidBody(pages[0])
	if err != nil || !strings.Contains(html, "Unlisted instance 99") {
		t.Fatal("stream model unrenderable")
	}
	if pages, err := readRaidPages(raidBrokenReader{}, nil, raidPrepConfig(t)); err == nil || len(pages) != 0 {
		t.Fatal("failed read returned page")
	}
}

type raidBrokenReader struct{}

func (raidBrokenReader) Read([]byte) (int, error) { return 0, errors.New("synthetic I/O failure") }

func TestRaidPageComparisonRereads(t *testing.T) {
	r := raidPageFixture()
	c := raidPrepConfig(t)
	scores := score.Score(r.Fights, nil, nil, c)
	path := filepath.Join(t.TempDir(), "prediction.json")
	for _, name := range []string{"First Officer", "Second Officer"} {
		data := []byte(`{"version":1,"officer":"` + name + `","top":["Bram-Oak-Hill-PvP-EU"],"bottom":["Alia-Stone-PvP-EU","Nobody-PvP-EU"]}`)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		comparison, err := raidComparisonFile(path, scores, r, c)
		if err != nil {
			t.Fatal(err)
		}
		p, err := buildRaidPage(r, logfmt.ParseReport{}, nil, c)
		if err != nil {
			t.Fatal(err)
		}
		p.Comparison = &comparison
		html, err := renderRaidBody(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{name, "agreement", "no rank: 17% measured", "not in this night", "Score's top 5 you did not name"} {
			if !strings.Contains(html, want) {
				t.Fatalf("missing %q", want)
			}
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, after) {
			t.Fatal("officer file changed")
		}
	}
	if err := os.WriteFile(path, []byte(`{"version":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := raidComparisonFile(path, scores, r, c); err == nil {
		t.Fatal("invalid comparison reused old file")
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 65537), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := raidComparisonFile(path, scores, r, c); err == nil {
		t.Fatal("oversized comparison accepted")
	}
}

// Opt-in, counts only: no path/name/identifier/raw log in failures or output.
func TestRaidPageRealLogs(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("private real logs not enabled")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("private folder unreadable")
	}
	logs := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		f, err := os.Open(filepath.Join(folder, entry.Name()))
		if err != nil {
			t.Fatal("private input unreadable")
		}
		pages, readErr := readRaidPages(f, nil, raidPrepConfig(t))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("private page pipeline failed")
		}
		logs++
		bosses, trash, ranked, provisional, unscored, deaths, wipes, rendered := 0, 0, 0, 0, 0, 0, 0, 0
		for _, p := range pages {
			html, err := renderRaidBody(p)
			if err != nil || len(html) == 0 {
				t.Fatal("private page render failed")
			}
			rendered++
			bosses += p.Bosses
			trash += p.Trash
			ranked += len(p.Ranked)
			provisional += len(p.Provisional)
			unscored += len(p.Unscored)
			deaths += len(p.Deaths)
			wipes += len(p.Wipes)
			if !p.Dungeon {
				t.Fatal("private dungeon classification changed")
			}
			for _, list := range [][]raidPlayerView{p.Ranked, p.Provisional, p.Unscored} {
				for _, row := range list {
					if row.Rank != "" {
						t.Fatal("private dungeon has a rank")
					}
				}
			}
		}
		if bosses != 13 || rendered == 0 {
			t.Fatal("private page count mismatch")
		}
		t.Logf("log %d: rendered_pages=%d bosses=%d trash=%d scored_unranked=%d provisional=%d not_enough=%d deaths=%d wipes=%d", logs, rendered, bosses, trash, ranked, provisional, unscored, deaths, wipes)
	}
	if logs != 2 {
		t.Fatal("expected two private logs")
	}
}
