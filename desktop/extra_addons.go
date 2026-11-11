package main

// Optional companion addons (RankUp, Forever Maps, AH Profit). They are separate products
// from separate GitHub repositories, installed and updated one at a time on an
// explicit click. Each is a single addon folder; nothing else in the game folder,
// no SavedVariables and no other addon is touched. The same trust rules as the
// Coach updater apply: exact HTTPS release URLs from a fixed repository, a
// SHA256SUMS.txt match, a safe archive, an exact TOC identity, WoW closed, a kept
// backup of the previous folder and no reinstall or downgrade.
import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type extraAddon struct {
	ID, Name, Repo, Folder, Title, Blurb string
}

// Fixed list: the folder, TOC title and repository are never taken from a
// release, the UI or an archive.
var extraAddons = []extraAddon{
	{ID: "rankup", Name: "RankUp", Repo: "buiden/RankUp", Folder: "RankUp", Title: "RankUp",
		Blurb: "Keeps your action bars on the highest spell rank you know."},
	{ID: "forevermaps", Name: "Forever Maps", Repo: "buiden/ForeverMaps", Folder: "ForeverMaps", Title: "Forever Maps",
		Blurb: "Directions to any place: running, flying and boat routes for your faction."},
	{ID: "ahprofit", Name: "AH Profit", Repo: "buiden/AHProfit", Folder: "AHProfit", Title: "AH Profit",
		Blurb: "Vendor price, auction house price and AH profit on every item tooltip."},
}

func extraAddonByID(id string) (extraAddon, bool) {
	for _, a := range extraAddons {
		if a.ID == id {
			return a, true
		}
	}
	return extraAddon{}, false
}

func (a extraAddon) latestURL() string {
	return "https://api.github.com/repos/" + a.Repo + "/releases/latest"
}
func extraLatestURL(raw string) bool {
	for _, a := range extraAddons {
		if raw == a.latestURL() {
			return true
		}
	}
	return false
}

func (a extraAddon) releasePrefix() string { return "https://github.com/" + a.Repo + "/releases/" }
func (a extraAddon) assetName(version string) string {
	return a.Folder + "-" + version + ".zip"
}

// ExtraAddonInfo is display data plus the release the user confirmed. Apply
// revalidates everything security-relevant against the fixed addon entry.
type ExtraAddonInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Blurb            string `json:"blurb"`
	InstalledVersion string `json:"installedVersion"`
	Version          string `json:"version"`
	Notes            string `json:"notes"`
	HTMLURL          string `json:"htmlUrl"`
	AssetURL         string `json:"-"`
	ChecksumURL      string `json:"-"`
	Available        bool   `json:"available"`
	Install          bool   `json:"install"` // not installed yet: offer "Install" instead of "Update"
	Status           string `json:"status"`
}

// InstalledExtraVersion reads <AddOns>/<Folder>/<Folder>.toc. Missing or
// unreadable metadata is "0.0.0" (not installed); apply separately refuses an
// existing folder it cannot identify.
func InstalledExtraVersion(game string, a extraAddon) string {
	state, err := extraInstalled(game, a)
	if err != nil || !state.exists {
		return "0.0.0"
	}
	return state.version.text
}

// ExtraInstalledProblem is non-empty when an <AddOns>/<Folder> exists but is
// not that addon (or its TOC can't be read); the UI shows it as unrecognized.
func ExtraInstalledProblem(game string, a extraAddon) string {
	if game == "" {
		return ""
	}
	if _, err := extraInstalled(game, a); err != nil {
		return err.Error()
	}
	return ""
}

func extraTarget(game string, a extraAddon) string {
	return filepath.Join(filepath.Clean(game), "Interface", "AddOns", a.Folder)
}

func extraInstalled(game string, a extraAddon) (addonInstalledState, error) {
	var state addonInstalledState
	state.version, _ = parseAddonVersion("0.0.0")
	if game == "" || !filepath.IsAbs(game) {
		return state, errors.New("select an absolute World of Warcraft game folder first")
	}
	target := extraTarget(game, a)
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := addonPlainDirectory(target); err != nil {
		return state, err
	}
	b, err := addonReadSmallFile(filepath.Join(target, a.Folder+".toc"))
	if err != nil {
		return state, fmt.Errorf("cannot identify the installed %s folder: %w", a.Name, err)
	}
	if err := extraCheckTitle(b, a); err != nil {
		return state, err
	}
	v, err := addonExactTOCVersion(b)
	if err != nil {
		return state, fmt.Errorf("cannot read the installed %s version: %w", a.Name, err)
	}
	return addonInstalledState{exists: true, info: info, toc: b, version: v}, nil
}

func extraCheckTitle(toc []byte, a extraAddon) error {
	meta, _, err := addonTOCMetadata(toc)
	if err != nil {
		return err
	}
	if meta["title"] != a.Title {
		return fmt.Errorf("the %s folder does not contain %s (TOC Title %q)", a.Folder, a.Name, meta["title"])
	}
	return nil
}

func CheckExtraAddon(ctx context.Context, id, current string) (ExtraAddonInfo, error) {
	return newAddonUpdater().checkExtra(ctx, id, current)
}

func (u *addonUpdater) checkExtra(ctx context.Context, id, current string) (ExtraAddonInfo, error) {
	a, ok := extraAddonByID(id)
	if !ok {
		return ExtraAddonInfo{}, errors.New("unknown addon")
	}
	info := ExtraAddonInfo{ID: a.ID, Name: a.Name, Blurb: a.Blurb, InstalledVersion: current}
	local, err := parseAddonVersion(current)
	if err != nil {
		return info, errors.New("invalid installed version")
	}
	b, err := u.fetch(ctx, a.latestURL(), 1<<20, a.Name+" release information")
	var httpErr *addonHTTPError
	if errors.As(err, &httpErr) && httpErr.status == http.StatusNotFound {
		// GitHub answers releases/latest with 404 when no stable release exists.
		info.Status = "No stable " + a.Name + " release is published."
		return info, nil
	}
	if err != nil {
		return info, err
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(b, &release); err != nil {
		return info, fmt.Errorf("GitHub returned invalid %s release information", a.Name)
	}
	remote, err := parseAddonVersion(release.Tag)
	if err != nil || release.Draft || release.Prerelease || release.Tag != "v"+remote.text {
		info.Status = "No stable " + a.Name + " release is published."
		return info, nil
	}
	info.Version, info.Notes = remote.text, release.Body
	info.HTMLURL = a.releasePrefix() + "tag/" + release.Tag
	wanted := a.assetName(remote.text)
	for _, asset := range release.Assets {
		switch asset.Name {
		case wanted:
			if info.AssetURL != "" {
				return ExtraAddonInfo{}, errors.New("GitHub release contains duplicate addon assets")
			}
			info.AssetURL = asset.URL
		case "SHA256SUMS.txt":
			if info.ChecksumURL != "" {
				return ExtraAddonInfo{}, errors.New("GitHub release contains duplicate checksum assets")
			}
			info.ChecksumURL = asset.URL
		}
	}
	if info.AssetURL == "" || info.ChecksumURL == "" {
		info.AssetURL, info.ChecksumURL = "", ""
		info.Status = "The " + a.Name + " " + remote.text + " release is missing " + wanted + " or SHA256SUMS.txt."
		return info, nil
	}
	if err := extraValidateURLs(a, info); err != nil {
		return ExtraAddonInfo{}, err
	}
	if !remote.newerThan(local) {
		info.AssetURL, info.ChecksumURL = "", ""
		info.Status = a.Name + " is up to date."
		return info, nil
	}
	info.Available, info.Install = true, current == "0.0.0"
	if info.Install {
		info.Status = a.Name + " " + remote.text + " is ready to install."
	} else {
		info.Status = a.Name + " " + remote.text + " is available."
	}
	return info, nil
}

func extraValidateURLs(a extraAddon, info ExtraAddonInfo) error {
	v, err := parseAddonVersion(info.Version)
	if err != nil || v.text != info.Version {
		return errors.New("invalid addon release version")
	}
	base := a.releasePrefix() + "download/v" + v.text + "/"
	if info.AssetURL != base+a.assetName(v.text) || info.ChecksumURL != base+"SHA256SUMS.txt" {
		return errors.New("release asset is not from the trusted GitHub repository and release")
	}
	return nil
}

var extraInstallMutex sync.Mutex

func ApplyExtraAddon(ctx context.Context, game string, info ExtraAddonInfo) (AddonUpdateResult, error) {
	return newAddonUpdater().applyExtra(ctx, game, info)
}

func (u *addonUpdater) applyExtra(ctx context.Context, game string, info ExtraAddonInfo) (result AddonUpdateResult, err error) {
	if !extraInstallMutex.TryLock() {
		return result, errors.New("another addon install is in progress")
	}
	defer extraInstallMutex.Unlock()
	defer func() {
		if err != nil && errors.Is(err, os.ErrPermission) {
			err = fmt.Errorf("installing needs write permission to the game folder; nothing else was changed: %w", err)
		}
	}()
	a, ok := extraAddonByID(info.ID)
	if !ok {
		return result, errors.New("unknown addon")
	}
	if !info.Available {
		return result, errors.New("no new version was offered; check again")
	}
	if err := extraValidateURLs(a, info); err != nil {
		return result, err
	}
	if err := u.requireStopped(ctx); err != nil {
		return result, err
	}
	if game == "" || !filepath.IsAbs(game) {
		return result, errors.New("select an absolute World of Warcraft game folder first")
	}
	game = filepath.Clean(game)
	addons := filepath.Join(game, "Interface", "AddOns")
	if err := addonPlainDirectory(addons); err != nil {
		return result, fmt.Errorf("invalid game addon folder: %w", err)
	}
	// Capture AddOns and every parent now; they are re-checked right before
	// the swap so a folder replaced by a link during the download can't
	// redirect the install outside the game folder (also for fresh installs).
	ancestors, err := extraAncestors(addons)
	if err != nil {
		return result, fmt.Errorf("invalid game addon folder: %w", err)
	}
	// Finish an earlier install that was interrupted mid-swap before reading
	// the installed state, so the offer is checked against the real folder.
	backups := filepath.Join(game, "RelaxCoachBackups")
	lockPath := filepath.Join(backups, ".extra-addon-update.lock")
	if addonPlainDirectory(backups) == nil && extraRecoverInterrupted(lockPath, backups, game) {
		os.Remove(lockPath)
	}
	before, err := extraInstalled(game, a)
	if err != nil {
		return result, err
	}
	remote, _ := parseAddonVersion(info.Version)
	if !remote.newerThan(before.version) {
		return result, errors.New("that version or a newer one is already installed; refusing to reinstall or downgrade")
	}
	sums, err := u.fetch(ctx, info.ChecksumURL, u.limits.checksum, "release checksum")
	if err != nil {
		return result, err
	}
	want, err := addonExpectedChecksum(sums, a.assetName(remote.text))
	if err != nil {
		return result, err
	}
	payload, err := u.fetch(ctx, info.AssetURL, u.limits.archive, a.Name+" archive")
	if err != nil {
		return result, err
	}
	if sha256.Sum256(payload) != want {
		return result, errors.New("SHA256 checksum mismatch; nothing was installed")
	}

	if err := os.Mkdir(backups, 0700); err != nil && !os.IsExist(err) {
		return result, err
	}
	if err := addonPlainDirectory(backups); err != nil {
		return result, err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) && extraStaleLock(lockPath, backups) {
		if rerr := os.Remove(lockPath); rerr == nil {
			lock, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		}
	}
	if os.IsExist(err) {
		return result, errors.New("another install or an unfinished one holds the lock; close other helpers and check RelaxCoachBackups")
	}
	if err != nil {
		return result, err
	}
	keepLock := false
	defer func() {
		lock.Close()
		if !keepLock {
			os.Remove(lockPath)
		}
	}()
	transaction, err := os.MkdirTemp(backups, a.Folder+"-"+time.Now().UTC().Format("20060102T150405Z")+"-")
	if err != nil {
		return result, err
	}
	fmt.Fprintln(lock, transaction)
	committed := false
	defer func() {
		if !committed && !keepLock {
			os.RemoveAll(transaction)
		}
	}()
	stage := filepath.Join(transaction, "staged")
	files, err := u.extractExtra(ctx, payload, stage, a, remote.text)
	if err != nil {
		return result, err
	}

	// Last checks right before the swap: WoW still closed, folder unchanged.
	if err := u.requireStopped(ctx); err != nil {
		return result, err
	}
	now, err := extraInstalled(game, a)
	if err != nil {
		return result, err
	}
	if now.exists != before.exists || (before.exists && (!os.SameFile(before.info, now.info) || !bytes.Equal(before.toc, now.toc))) {
		return result, errors.New("the installed addon changed during download; check again")
	}
	if err := addonCheckAncestors(ancestors); err != nil {
		return result, fmt.Errorf("the game addon folder changed during download; nothing was changed: %w", err)
	}
	target := extraTarget(game, a)
	backup := filepath.Join(transaction, "previous")
	if before.exists {
		if err := u.rename(target, backup); err != nil {
			return result, fmt.Errorf("could not move the old version aside; nothing was changed: %w", err)
		}
	}
	if err := u.rename(filepath.Join(stage, a.Folder), target); err != nil {
		if before.exists {
			if rerr := u.rename(backup, target); rerr != nil {
				keepLock = true
				return result, fmt.Errorf("install failed and the old version could not be put back; it is kept at %s: %w", backup, errors.Join(err, rerr))
			}
		}
		return result, fmt.Errorf("install failed; the previous version was restored: %w", err)
	}
	committed = true
	os.RemoveAll(stage)
	result = AddonUpdateResult{Version: remote.text, UpdatedFiles: files}
	if before.exists {
		result.BackupPath = backup
		result.Message = a.Name + " updated to " + remote.text + ". The previous version is backed up. /reload or restart WoW to load it."
	} else {
		os.RemoveAll(transaction)
		result.Message = a.Name + " " + remote.text + " installed. Enable it in the AddOns list if needed, then start WoW."
	}
	return result, nil
}

// extraLockStaleAfter is longer than the whole apply timeout, so a lock this
// old cannot belong to an install that is still running.
const extraLockStaleAfter = 15 * time.Minute

// extraStaleLock reports whether a leftover lock (for example after a crash or
// power loss) can be cleared safely: it is an old regular file and the
// transaction it names holds no moved-aside previous version. A lock whose
// transaction still has a "previous" folder is kept, because that folder may be
// the only copy of the old addon.
func extraStaleLock(lockPath, backups string) bool {
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) < extraLockStaleAfter {
		return false
	}
	b, err := addonReadSmallFile(lockPath)
	if err != nil {
		return false
	}
	transaction := filepath.Clean(strings.TrimSpace(string(b)))
	if transaction == "." {
		return true // crashed before the transaction path was written
	}
	if filepath.Dir(transaction) != filepath.Clean(backups) {
		return false
	}
	if _, err := os.Lstat(filepath.Join(transaction, "previous")); !os.IsNotExist(err) {
		return false
	}
	return true
}

// extraAncestors records the identity of dir and each of its parents.
func extraAncestors(dir string) (map[string]os.FileInfo, error) {
	ancestors := map[string]os.FileInfo{}
	for p := dir; ; p = filepath.Dir(p) {
		info, err := addonDirectoryIdentity(p)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, fmt.Errorf("missing update directory: %s", p)
		}
		ancestors[p] = info
		if filepath.Dir(p) == p {
			return ancestors, nil
		}
	}
}

// extraRecoverInterrupted finishes an install that died mid-swap (crash, power
// loss, or a failed rollback) so the lock does not block every later install.
// It only acts on an old lock whose transaction under backups belongs to a
// known extra addon and still holds the moved-aside "previous" folder:
//   - target missing: put the identified previous version back in place;
//   - target present: leave it and the backup alone.
//
// Either way the transaction folder is kept, and true means the lock may go.
func extraRecoverInterrupted(lockPath, backups, game string) bool {
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) < extraLockStaleAfter {
		return false
	}
	b, err := addonReadSmallFile(lockPath)
	if err != nil {
		return false
	}
	transaction := filepath.Clean(strings.TrimSpace(string(b)))
	if transaction == "." || filepath.Dir(transaction) != filepath.Clean(backups) {
		return false
	}
	var a extraAddon
	found := false
	for _, e := range extraAddons {
		if strings.HasPrefix(filepath.Base(transaction), e.Folder+"-") {
			a, found = e, true
			break
		}
	}
	if !found {
		return false
	}
	previous := filepath.Join(transaction, "previous")
	if err := addonPlainDirectory(previous); err != nil {
		return false
	}
	target := extraTarget(game, a)
	if _, err := os.Lstat(target); err == nil {
		return true // new or other version in place; backup stays in transaction
	} else if !os.IsNotExist(err) {
		return false
	}
	toc, err := addonReadSmallFile(filepath.Join(previous, a.Folder+".toc"))
	if err != nil || extraCheckTitle(toc, a) != nil {
		return false
	}
	if err := addonPlainDirectory(filepath.Dir(target)); err != nil {
		return false
	}
	return os.Rename(previous, target) == nil
}

// extractExtra writes <stage>/<Folder>/... from a ZIP whose every entry is
// under <Folder>/, and validates the TOC identity and load list.
func (u *addonUpdater) extractExtra(ctx context.Context, payload []byte, stage string, a extraAddon, version string) (int, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return 0, fmt.Errorf("invalid addon ZIP: %w", err)
	}
	if len(reader.File) > u.limits.entries {
		return 0, errors.New("addon archive exceeds the file-count limit")
	}
	prefix := a.Folder + "/"
	seen := map[string]bool{}
	var expanded int64
	var toc []byte
	count := 0
	for _, f := range reader.File {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if err := addonSafeZipName(f.Name); err != nil {
			return 0, err
		}
		name := strings.TrimSuffix(f.Name, "/")
		if name != a.Folder && !strings.HasPrefix(name, prefix) {
			return 0, fmt.Errorf("archive entry %q is outside the %s folder", f.Name, a.Folder)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return 0, fmt.Errorf("duplicate or case-colliding archive path %q", f.Name)
		}
		seen[key] = true
		dest := filepath.Join(stage, filepath.FromSlash(name))
		mode := f.Mode()
		if mode.IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return 0, err
			}
			continue
		}
		if !mode.IsRegular() {
			return 0, fmt.Errorf("archive entry %q is not a regular file", f.Name)
		}
		if f.UncompressedSize64 > uint64(u.limits.file) {
			return 0, fmt.Errorf("archive entry %q is too large", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return 0, err
		}
		rc, err := f.Open()
		if err != nil {
			return 0, err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			rc.Close()
			return 0, err
		}
		n, err := io.Copy(out, io.LimitReader(rc, u.limits.file+1))
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return 0, err
		}
		expanded += n
		if n > u.limits.file || expanded > u.limits.expanded {
			return 0, errors.New("addon archive exceeds the size limit")
		}
		if n != int64(f.UncompressedSize64) {
			return 0, fmt.Errorf("archive entry %q has the wrong size", f.Name)
		}
		if strings.EqualFold(name, prefix+a.Folder+".toc") {
			if name != prefix+a.Folder+".toc" {
				return 0, errors.New("addon TOC filename has the wrong case")
			}
			if toc, err = addonReadSmallFile(dest); err != nil {
				return 0, err
			}
		}
		count++
	}
	if toc == nil {
		return 0, fmt.Errorf("archive has no %s%s.toc", prefix, a.Folder)
	}
	if err := extraCheckTitle(toc, a); err != nil {
		return 0, err
	}
	v, err := addonExactTOCVersion(toc)
	if err != nil || v.text != version {
		return 0, errors.New("the TOC Version does not match the release version")
	}
	_, loads, err := addonTOCMetadata(toc)
	if err != nil {
		return 0, err
	}
	if len(loads) == 0 {
		return 0, errors.New("addon TOC loads no files")
	}
	for _, l := range loads {
		rel := strings.ReplaceAll(l, "\\", "/")
		if err := addonSafeZipName(rel); err != nil || !seen[strings.ToLower(prefix+rel)] {
			return 0, fmt.Errorf("addon TOC loads a missing or unsafe file %q", l)
		}
	}
	return count, nil
}
