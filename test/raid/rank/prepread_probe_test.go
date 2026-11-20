// Overlay into the real prep reader; fixtures come from the current Lua writer.
package prepread

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRankPrepRetained(t *testing.T) {
	for _, size := range []string{"96", "4096", "4097"} {
		raw, err := os.ReadFile(filepath.Join(os.Getenv("RANK_SAVES"), size+".lua"))
		if err != nil {
			t.Fatal(err)
		}
		rows, summary := Parse(raw, 1800000000, 1800000100)
		if len(rows) != 1 || len(summary.Warnings) != 0 || len(summary.Skipped) != 0 {
			t.Fatalf("guild bytes=%s: prep rows=%d warnings=%v skipped=%v", size, len(rows), summary.Warnings, summary.Skipped)
		}
		if rows[0].Name != "Alia-Stone-PvP-EU" || rows[0].Stamped != 1800000000 {
			t.Fatal("existing preparation identity or receipt changed")
		}
		t.Logf("guild bytes=%s: existing preparation retained without warnings", size)
	}
}
