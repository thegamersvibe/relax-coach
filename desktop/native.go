package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

var reportTemplate = template.Must(template.New("report.html").Funcs(funcs).ParseFS(assets, "report.html"))

// Only this template-owned fragment may be inserted into the native report panel.
// SavedVariables remain ordinary escaped text; they are never template.HTML/JS.
func renderReportBody(v View) (string, error) {
	var out strings.Builder
	err := reportTemplate.ExecuteTemplate(&out, "report-body", v)
	return out.String(), err
}

func nativeToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func renderNativeShell(v View, disabled bool, token string) (string, error) {
	nonce, err := nativeToken()
	if err != nil {
		return "", err
	}
	logo, err := assets.ReadFile("logo.png")
	if err != nil {
		return "", err
	}
	script, err := assets.ReadFile("native_shell.js")
	if err != nil {
		return "", err
	}
	tpl, err := template.New("native_shell.html").Funcs(funcs).ParseFS(assets, "report.html", "farm.html", "native_shell.html")
	if err != nil {
		return "", err
	}
	data := struct {
		View                           View
		UpdatesDisabled                bool
		Nonce, Token, InstalledVersion string
		AppVersion, AppUpdateStatus    string
		Logo                           template.URL
		Script                         template.JS
	}{View: v, UpdatesDisabled: disabled, Nonce: nonce, Token: token, InstalledVersion: InstalledCoachVersion(v.Game), AppVersion: Version, AppUpdateStatus: currentAppUpdateStatus(), Logo: template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(logo)), Script: template.JS(script)}
	var out strings.Builder
	err = tpl.ExecuteTemplate(&out, "native_shell.html", data)
	return out.String(), err
}

// Only documentation and this project's release pages may leave the app. No
// executable/file/custom schemes, credentials, alternate ports or lookalike hosts.
func validateExternalURL(raw string) error {
	denied := fmt.Errorf("this link is not an allowed HTTPS documentation or release URL")
	if len(raw) > 2048 || strings.IndexFunc(raw, func(r rune) bool { return r <= 32 || r == 127 }) >= 0 || strings.ContainsAny(raw, "\\") {
		return denied
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || (u.Port() != "" && u.Port() != "443") {
		return denied
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == ".." || part == "." {
			return denied
		}
	}
	switch u.Hostname() {
	case "github.com":
		if u.Path != "/"+Repo && !strings.HasPrefix(u.Path, "/"+Repo+"/") {
			return denied
		}
	case "www.wowhead.com", "wowhead.com", "www.icy-veins.com", "icy-veins.com":
	default:
		return denied
	}
	return nil
}
