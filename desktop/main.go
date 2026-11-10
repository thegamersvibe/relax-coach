// Relax Coach native Windows application.
// Reads only ForeverCoach SavedVariables. No memory reading or game automation.
// Diagnostic HTTP is opt-in; the normal UI uses direct in-process Go bindings.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var Version = "1.5.0" // May be overridden with -X main.Version for private verification builds.
const Repo = "thegamersvibe/relax-coach"
const RulesSchema = 1

//go:embed report.html logo.png native_shell.html native_shell.js
var assets embed.FS

type Finding struct {
	Impact   float64
	Area     string
	Text     string
	Fix      string
	Where    string
	Estimate bool
}

type CharView struct {
	Key, Name, Realm, Class, Race, Faction, Spec, SpecSource string
	Level                                                    int
	MaxLevel                                                 bool
	XPPct, RestedPct                                         int
	XPPerHour                                                int
	Updated                                                  time.Time
	Findings                                                 []Finding
	Sessions                                                 int
	Deaths                                                   []string
	Money                                                    string
	ClassColor                                               string
	Zone, Guide, GuideStep                                   string
	AddonError                                               string
	Stale                                                    []string
	Slots                                                    []SlotView
	GearStatus, GearNote                                     string
	GearAssessed, GearUnknown, GearEmpty, GearUpgrades       int
	Owned                                                    []OwnedView
	Talents                                                  []TalentView
	TalentSteps                                              []TalentStep
	TalentSource, TalentURL                                  string
	Dungeons                                                 []DungeonView
	Procs                                                    []ProcView
	Stats                                                    StatView
	Counts                                                   map[string]int
}

type SlotView struct {
	Name, Item, BestName, BestHow, BestSource string
	BestWhere, Status, Note                   string
	Equipped, Weapon                          bool
	GapPct                                    int
}

type TalentView struct {
	Name  string
	Spent int
	Pct   int
}

type ProcView struct {
	Name          string
	Used, Windows int
	UsedPct       int
}

type StatView struct {
	PlayMin, ActiveMin, Deaths                                 int
	CombatPct, DowntimePct, DeadPct, AFKPct, TaxiPct, OtherPct int
	DeathsPerHour                                              string
	XPToLevel, MinToLevel                                      int
	HasData                                                    bool
}

type View struct {
	Chars                       []CharView
	SVPath, Error, RulesVersion string
	Warning                     string
	Account                     string
	ConfigWarning               string
	Accounts                    []AccountChoice
	SaveTime                    time.Time
	Game                        string
	NoGame                      bool
	UpdateNote, AppVersion      string
	Loaded                      time.Time
}

type App struct {
	game, svPath, addonDir string
	mu                     sync.Mutex
	view                   View
	modTime                time.Time
	account                string
	accounts               []AccountChoice
	configWarning          string
	canAutoSelect          bool
}

func main() {
	mode, id, guiArgs, modeErr := parseAppMode(os.Args[1:])
	if modeErr != nil {
		log.Print(modeErr)
		os.Exit(1)
	}
	if mode == "worker" {
		if err := appPlatformWorker(id); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	gameFlag := flag.String("game", "", `game folder (the _forever_ folder); default: auto-detect`)
	port := flag.Int("port", 47321, "diagnostic HTTP port (only with -headless or -no-open)")
	noOpen := flag.Bool("no-open", false, "diagnostic HTTP mode; never launch a browser")
	headless := flag.Bool("headless", false, "diagnostic HTTP mode without an application window")
	noUpdate := flag.Bool("no-update", false, "disable app and addon release checks and updates")
	once := flag.Bool("once", false, "print the report to the console and exit without updating")
	_ = flag.CommandLine.Parse(guiArgs)
	if flag.NArg() != 0 {
		log.Print("unexpected positional arguments")
		os.Exit(1)
	}
	if !*once && !*headless && !*noOpen {
		initAppUpdateRuntime(guiArgs)
		if mode == "start" {
			if err := initAppStartup(id); err != nil {
				log.Print(err)
				os.Exit(1)
			}
		}
	}
	game := NormalizeGame(*gameFlag)
	if game == "" {
		game = findGame()
	}
	app := &App{}
	app.setGame(game)
	app.reload()
	if *once {
		printConsole(app.snapshot())
		return
	}
	// The window owns the process. HTTP is an explicit diagnostic opt-in, on every OS.
	if *headless || *noOpen {
		if err := serveDiagnostic(app, *port); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := runNative(app, *noUpdate); err != nil {
		showNativeError(err)
		log.Print(err)
		os.Exit(1)
	}
}

func (a *App) snapshot() View {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.view
}

func (a *App) setGame(game string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.game = game
	a.addonDir = ""
	a.account, a.configWarning = "", ""
	remembered, err := savedAccount(game)
	a.canAutoSelect = err == nil
	if err != nil {
		a.configWarning = "Could not read remembered account: " + err.Error()
	} else {
		a.account = remembered
	}
	if game != "" {
		a.addonDir = filepath.Join(game, "Interface", "AddOns", "ForeverCoach")
	}
	a.rescanAccountsLocked()
	a.modTime = time.Time{}
	// Do not carry another game's last-good report across a folder change.
	a.view = View{AppVersion: Version, Game: game, NoGame: game == ""}
}

// Refresh, folder selection, and snapshots synchronize all report state. There is
// no lifetime timer: a minimized native window can remain open indefinitely.
func (a *App) refresh() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rescanAccountsLocked()
	a.reloadLocked()
}

func (a *App) reload() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reloadLocked()
}

func (a *App) reloadLocked() {
	prev := a.view
	v := View{SVPath: a.svPath, AppVersion: Version, Loaded: time.Now(), UpdateNote: prev.UpdateNote, Game: a.game, NoGame: a.game == "", Account: a.account, Accounts: a.accounts, ConfigWarning: a.configWarning}
	v.RulesVersion = installedRulesVersion(a.addonDir)
	if a.game == "" {
		v.Error = "Couldn't find WoW Forever on this PC. Choose or paste your game folder above."
	} else if a.account == "" && len(a.accounts) > 0 {
		v.Error = "Choose an account above to view its characters. Accounts are never combined."
	} else if a.svPath == "" {
		v.Error = "No Relax Coach data yet in " + a.game + ". Install the addon, log in, then /reload."
	} else {
		b, st, err := a.readSelectedSave()
		var vars map[string]any
		if err == nil {
			v.SaveTime = st.ModTime()
			vars, err = ParseSavedVariables(string(b))
		}
		db := M(vars["ForeverCoachDB"])
		if err == nil && db == nil {
			err = fmt.Errorf("no ForeverCoachDB in %s", a.svPath)
		}
		if err == nil && N(db["schema"]) > 1 {
			err = fmt.Errorf("this save was written by a newer Relax Coach addon; update the addon")
		}
		if err != nil {
			v.Error = err.Error()
		} else {
			a.modTime = st.ModTime()
			v.Chars = buildChars(db)
		}
	}
	if v.Error != "" && prev.SVPath != "" && prev.Error == "" && prev.Game == v.Game && prev.Account == v.Account {
		prev.Accounts = v.Accounts
		prev.ConfigWarning = v.ConfigWarning
		prev.Warning = "Latest save couldn't be read yet, showing the previous one (" + v.Error + ")"
		prev.Loaded = time.Now()
		a.view = prev
		return
	}
	a.view = v
}

func buildChars(db map[string]any) []CharView {
	var out []CharView
	for key, raw := range M(db["chars"]) {
		c := M(raw)
		s := M(c["snapshot"])
		g := M(c["gear"])
		cv := CharView{
			Key: key, Name: S(s["name"]), Realm: S(s["realm"]), Class: S(s["class"]), Race: S(s["race"]),
			Faction: S(s["faction"]), Level: int(N(s["level"])), Spec: S(g["spec"]), SpecSource: S(g["specSource"]),
			XPPerHour: int(N(c["xpPerHour"])), Updated: time.Unix(int64(N(c["updated"])), 0),
		}
		if mx := N(s["xpMax"]); mx > 0 {
			cv.XPPct = int(N(s["xp"]) / mx * 100)
			cv.RestedPct = int(N(s["rested"]) / mx * 100)
		}
		for _, f := range List(c["findings"]) {
			fm := M(f)
			cv.Findings = append(cv.Findings, Finding{N(fm["impact"]), S(fm["area"]), S(fm["text"]), S(fm["fix"]), S(fm["where"]), fm["estimate"] == true})
		}
		sess := List(c["sessions"])
		cv.Sessions = len(sess)
		for i := len(sess) - 1; i >= 0 && len(cv.Deaths) < 8; i-- {
			for _, d := range List(M(sess[i])["deaths"]) {
				dm := M(d)
				line := fmt.Sprintf("Level %s, %s", S(dm["level"]), S(dm["zone"]))
				if t := int64(N(dm["time"])); t > 0 {
					line += " · " + ago(time.Unix(t, 0))
				}
				cv.Deaths = append(cv.Deaths, line)
			}
		}
		enrich(&cv, c, s, g, sess)
		out = append(out, cv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out
}

func ago(t time.Time) string {
	if t.Unix() <= 0 {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return t.Format("2006-01-02")
}

func printConsole(v View) {
	if v.Error != "" {
		fmt.Println(v.Error)
	}
	for _, c := range v.Chars {
		fmt.Printf("\n%s (%s %s %s, level %d, spec %s) updated %s\n", c.Key, c.Race, c.Class, c.Faction, c.Level, c.Spec, ago(c.Updated))
		for i, f := range c.Findings {
			if i >= 10 {
				break
			}
			fmt.Printf("  [%s] %s\n      %s\n", f.Area, f.Text, f.Fix)
		}
	}
	if v.UpdateNote != "" {
		fmt.Println("\n" + v.UpdateNote)
	}
	b, _ := json.Marshal(struct{ Rules string }{v.RulesVersion})
	fmt.Println(string(b))
}
