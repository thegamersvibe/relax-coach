package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Keep native JS guards executable by the named Go mutation runner. The HTML
// comes from the actual embedded assets, including Go build-overlay mutations.
func runRaidUI(t *testing.T, script, pattern string) {
	t.Helper()
	t.Setenv("NATIVE_HTML_OUT", filepath.Join(t.TempDir(), "native.html"))
	TestNativeShellIsSelfContainedAndLockedDown(t)
	TestRaidPageOpenEmbedded(t)
	if script == "native_raid_ui_test.cjs" {
		TestRaidLockRenderFixture(t)
		TestRaidPageEvidence(t)
		TestRaidPageWipesAndProblems(t)
		TestRaidPageNamesIsolated(t)
		TestRaidPageScreenDisplay(t)
		TestRaidPageTinyDecimal(t)
	}
	modules, err := filepath.Abs(filepath.Join("..", ".github", "node_modules"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--test", "--test-name-pattern", pattern, script)
	cmd.Env = append(os.Environ(), "NODE_PATH="+modules)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native raid UI: %v\n%s", err, out)
	}
	t.Log(string(out))
}

func TestRaidLockReplyOrdering(t *testing.T) {
	runRaidUI(t, "native_raid_order_ui_test.cjs", "lock ordering")
}
func TestRaidPageRouteErrors(t *testing.T) {
	runRaidUI(t, "native_raid_order_ui_test.cjs", "route error")
}
func TestRaidPageDisplayInsertion(t *testing.T) {
	runRaidUI(t, "native_raid_ui_test.cjs", "every name|display|tiny measured")
}

func TestRaidLockPreferenceReply(t *testing.T) {
	runRaidUI(t, "native_raid_pref_ui_test.cjs", "preference reply")
}
