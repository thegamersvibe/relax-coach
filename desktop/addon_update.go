package main

// The addon updater is intentionally independent of report/rules updates. It
// never executes downloads, replaces the desktop helper, or writes SavedVariables.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const addonLatestURL = "https://api.github.com/repos/thegamersvibe/relax-coach/releases/latest"
const addonReleasePrefix = "https://github.com/thegamersvibe/relax-coach/releases/"
const addonMinimumVersion = "1.4.0"
const addonGuildMinimumVersion = "1.4.2"

// From 1.5.0 the combined bundle also carries Relax Quest Planner. Apps before
// 1.5.0 refuse that folder (their install scope is fixed), so the bundle has a
// new asset name: an older app then reports an app-only release and asks for
// Update App first, before anything is downloaded.
const addonBundleMinimumVersion = "1.5.0"

func addonBundleRelease(version string) bool {
	v, err := parseAddonVersion(version)
	minimum, _ := parseAddonVersion(addonBundleMinimumVersion)
	return err == nil && !minimum.newerThan(v)
}

func addonGuildRelease(version string) bool {
	v, err := parseAddonVersion(version)
	minimum, _ := parseAddonVersion(addonGuildMinimumVersion)
	return err == nil && !minimum.newerThan(v)
}

func addonAssetName(version string) string {
	if addonBundleRelease(version) {
		return "RelaxCoach-Bundle-" + version + ".zip"
	}
	if addonGuildRelease(version) {
		return "RelaxCoach-Guild-" + version + ".zip"
	}
	return "RelaxCoach-" + version + ".zip"
}

// AddonReleaseInfo is safe to display, but ApplyAddonRelease revalidates every
// security-relevant field: JSON from a caller is not an authorization token.
type AddonReleaseInfo struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	Notes       string `json:"notes"`
	HTMLURL     string `json:"htmlUrl"`
	AssetName   string `json:"assetName"`
	AssetURL    string `json:"assetUrl"`
	ChecksumURL string `json:"checksumUrl"`
	Available   bool   `json:"available"`
	// Repair: the same version as the installed Coach, offered only because
	// the installed bundle is incomplete (a missing folder or an interrupted
	// install). ApplyAddonRelease re-checks that before installing.
	Repair bool `json:"repair"`
	// sameVersion: for a release equal to the installed Coach, the full
	// release, so the app can offer it as a repair. Never shown to the UI.
	sameVersion *AddonReleaseInfo
}

type addonUpdater struct {
	client  *http.Client
	running func(context.Context) (bool, error)
	rename  func(string, string) error
	limits  addonLimits
}

type addonLimits struct {
	archive, checksum, expanded, file int64
	entries                           int
}

func newAddonUpdater() *addonUpdater {
	return &addonUpdater{
		rename:  os.Rename,
		running: addonGameRunning,
		limits:  addonLimits{archive: 64 << 20, checksum: 64 << 10, expanded: 128 << 20, file: 64 << 20, entries: 2048},
		client: &http.Client{
			Timeout:       90 * time.Second,
			CheckRedirect: addonRedirect,
		}}
}

// CheckAddonRelease checks only the public stable channel. A missing compatible
// asset or an older release is a normal, explicitly described no-update result.
func CheckAddonRelease(ctx context.Context, current string) (AddonReleaseInfo, error) {
	return newAddonUpdater().check(ctx, current)
}

var addonSemverRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

type addonSemver struct {
	text string
	nums [3]uint64
}

func parseAddonVersion(s string) (addonSemver, error) {
	var v addonSemver
	s = strings.TrimPrefix(s, "v")
	if len(s) > 128 {
		return v, errors.New("version is too long")
	}
	m := addonSemverRE.FindStringSubmatch(s)
	if m == nil {
		return v, fmt.Errorf("%q is not a stable semantic version", s)
	}
	v.text = s
	for i := range v.nums {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return addonSemver{}, errors.New("version number is too large")
		}
		v.nums[i] = n
	}
	return v, nil
}

func (v addonSemver) newerThan(other addonSemver) bool {
	for i, n := range v.nums {
		if n != other.nums[i] {
			return n > other.nums[i]
		}
	}
	return false // SemVer build metadata never changes precedence.
}

func addonNoUpdate(info AddonReleaseInfo, reason string) AddonReleaseInfo {
	info.Available = false
	info.Version = ""
	info.AssetName, info.AssetURL, info.ChecksumURL = "", "", ""
	info.Notes = "No compatible new addon update is available. " + reason
	return info
}

func (u *addonUpdater) check(ctx context.Context, current string) (AddonReleaseInfo, error) {
	var info AddonReleaseInfo
	local, err := parseAddonVersion(current)
	if err != nil {
		return info, fmt.Errorf("cannot check addon updates: invalid current version: %w", err)
	}
	b, err := u.fetch(ctx, addonLatestURL, 1<<20, "release information")
	if err != nil {
		return info, err
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Name       string `json:"name"`
		Body       string `json:"body"`
		HTMLURL    string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(b, &release); err != nil {
		return info, fmt.Errorf("GitHub returned invalid release information: %w", err)
	}
	info.Name, info.Notes = release.Name, release.Body
	remote, err := parseAddonVersion(release.Tag)
	if err != nil || release.Draft || release.Prerelease {
		return addonNoUpdate(info, "The public release is not a stable Relax Coach package."), nil
	}
	info.Version = remote.text
	info.HTMLURL = addonReleasePrefix + "tag/" + release.Tag
	minimum, _ := parseAddonVersion(addonMinimumVersion)
	if minimum.newerThan(remote) {
		return addonNoUpdate(info, "Your installed version will not be downgraded."), nil
	}
	wanted := addonAssetName(remote.text)
	foundAsset, foundChecksum, foundApp := false, false, false
	for _, asset := range release.Assets {
		switch asset.Name {
		case "RelaxCoach.exe":
			foundApp = true
		case wanted:
			if foundAsset {
				return info, errors.New("GitHub release contains duplicate addon assets")
			}
			foundAsset = true
			info.AssetName, info.AssetURL = asset.Name, asset.URL
		case "SHA256SUMS.txt":
			if foundChecksum {
				return info, errors.New("GitHub release contains duplicate checksum assets")
			}
			foundChecksum = true
			info.ChecksumURL = asset.URL
		}
	}
	if !foundAsset {
		reason := "The public release is missing " + wanted + "."
		if foundApp {
			reason = "This is an app-only release for this updater; no compatible addon bundle (" + wanted + ") is included. Use Update App separately."
		}
		return addonNoUpdate(info, reason), nil
	}
	if !foundChecksum {
		return addonNoUpdate(info, "The addon package cannot be verified: SHA256SUMS.txt is missing."), nil
	}
	if err := validateAddonAssetURL(info.AssetURL, release.Tag, wanted); err != nil {
		return AddonReleaseInfo{}, err
	}
	if err := validateAddonAssetURL(info.ChecksumURL, release.Tag, "SHA256SUMS.txt"); err != nil {
		return AddonReleaseInfo{}, err
	}
	if !remote.newerThan(local) {
		none := addonNoUpdate(info, "Your installed version is current or newer and will not be downgraded.")
		if !local.newerThan(remote) {
			same := info
			same.Available, same.Repair = true, true
			none.sameVersion = &same
		}
		return none, nil
	}
	info.Available = true
	return info, nil
}

func validateAddonAssetURL(raw, tag, name string) error {
	// Exact URLs, not suffix matches: reject alternate owners, encodings, query
	// tricks, credentials, ports, fragments and another release's checksum.
	if raw != addonReleasePrefix+"download/"+tag+"/"+name {
		return errors.New("release asset is not from the trusted GitHub repository and release")
	}
	return nil
}

func addonRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many GitHub download redirects")
	}
	v := req.URL
	if v.Scheme != "https" || v.User != nil || v.Port() != "" || v.Fragment != "" {
		return errors.New("refusing an insecure GitHub download redirect")
	}
	if len(via) == 0 {
		return errors.New("unexpected download redirect")
	}
	original := via[0].URL
	if original.Host == "api.github.com" {
		if v.String() != addonLatestURL && !extraLatestURL(v.String()) {
			return errors.New("refusing a redirect outside the release API")
		}
		return nil
	}
	switch v.Host {
	case "release-assets.githubusercontent.com", "objects.githubusercontent.com", "github-releases.githubusercontent.com":
		return nil // GitHub's exact HTTPS release-asset CDN hosts (signed queries are allowed).
	case "github.com":
		if v.String() == original.String() {
			return nil
		}
	}
	return errors.New("refusing a redirect outside trusted GitHub release-asset hosts")
}

// addonHTTPError keeps the status code so callers can treat a missing
// release (404) differently from a transient failure.
type addonHTTPError struct {
	status  int
	purpose string
}

func (e *addonHTTPError) Error() string {
	return fmt.Sprintf("GitHub %s request failed (HTTP %d); try again later. Reports remain usable offline", e.purpose, e.status)
}

func (u *addonUpdater) fetch(ctx context.Context, raw string, limit int64, purpose string) ([]byte, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" {
		return nil, errors.New("addon downloads require a trusted HTTPS URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RelaxCoach-AddonUpdater")
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream")
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not download %s; reports remain usable offline: %w", purpose, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0") {
		return nil, errors.New("GitHub rate limit reached; try again later. Reports remain usable offline")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &addonHTTPError{status: resp.StatusCode, purpose: purpose}
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("%s exceeds the download size limit", purpose)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", purpose, err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s exceeds the download size limit", purpose)
	}
	return b, nil
}
