package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Private render gate: AHPROFIT_SV_FILE (a real AHProfit.lua, outside the
// repo) and FARM_SHELL_OUT (where to write a full shell page with the
// Farming view open) are both required; otherwise this skips. Optional:
// AHPROFIT_CHARS="Name|Realm|Faction,..." gives the Coach characters that
// tell bank characters' houses apart. No private data lives in the repo.
func TestFarmPrivateRealSaveRender(t *testing.T) {
	sv, out := os.Getenv("AHPROFIT_SV_FILE"), os.Getenv("FARM_SHELL_OUT")
	if sv == "" || out == "" {
		t.Skip("set AHPROFIT_SV_FILE and FARM_SHELL_OUT for the private real-save render")
	}
	data, err := os.ReadFile(sv)
	if err != nil {
		t.Fatal(err)
	}
	var chars []CharView
	for _, c := range strings.Split(os.Getenv("AHPROFIT_CHARS"), ",") {
		if p := strings.Split(c, "|"); len(p) == 3 {
			chars = append(chars, CharView{Name: p[0], Realm: p[1], Faction: p[2], Updated: time.Now()})
		}
	}
	start := time.Now()
	fv, err := buildFarm(data, chars, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed %d bytes in %v: %d houses, %d bank rows, %d maps, %d auction stats, %d listing sets", len(data), time.Since(start), len(fv.Houses), len(fv.Sell), len(fv.Gathering), len(fv.Auctions), len(fv.Listings))
	html, err := renderNativeShell(View{Game: `C:\WoW\_forever_`, Account: "ACCOUNT", Farm: fv, AppVersion: Version}, false, "t")
	if err != nil {
		t.Fatal(err)
	}
	// Open the Farming view for the screenshot.
	html = strings.Replace(html, `<section id="report-view">`, `<section id="report-view" hidden>`, 1)
	html = strings.Replace(html, `<section id="farm-view" hidden>`, `<section id="farm-view">`, 1)
	html = strings.Replace(html, `<button id="nav-report" class="navbtn active" aria-current="page">`, `<button id="nav-report" class="navbtn">`, 1)
	html = strings.Replace(html, `<button id="nav-farm" class="navbtn">`, `<button id="nav-farm" class="navbtn active" aria-current="page">`, 1)
	if err := os.WriteFile(out, []byte(html), 0600); err != nil {
		t.Fatal(err)
	}
}
