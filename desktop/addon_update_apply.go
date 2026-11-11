package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type AddonUpdateResult struct {
	Version      string `json:"version"`
	BackupPath   string `json:"backupPath"`
	UpdatedFiles int    `json:"updatedFiles"`
	Message      string `json:"message"`
}

var addonInstallMutex sync.Mutex

// ApplyAddonRelease updates only the fixed addon/runtime trees. It does
// not update the running helper. The updater must never be invoked implicitly
// while generating a report: installing is a separate, explicit user action.
func ApplyAddonRelease(ctx context.Context, game string, release AddonReleaseInfo) (AddonUpdateResult, error) {
	return newAddonUpdater().apply(ctx, game, release)
}

func validateAddonRelease(info AddonReleaseInfo) error {
	v, err := parseAddonVersion(info.Version)
	if err != nil || v.text != info.Version {
		return errors.New("invalid stable addon release version")
	}
	minimum, _ := parseAddonVersion(addonMinimumVersion)
	if minimum.newerThan(v) {
		return errors.New("incompatible old addon release; refusing a downgrade")
	}
	if !info.Available || info.AssetName != addonAssetName(info.Version) {
		return errors.New("no compatible new addon update was selected")
	}
	prefix := addonReleasePrefix + "download/"
	if !strings.HasPrefix(info.AssetURL, prefix) {
		return errors.New("addon asset is outside the trusted GitHub release channel")
	}
	parts := strings.Split(strings.TrimPrefix(info.AssetURL, prefix), "/")
	if len(parts) != 2 {
		return errors.New("invalid trusted release asset path")
	}
	tag, err := parseAddonVersion(parts[0])
	if err != nil || tag.text != v.text {
		return errors.New("release tag and addon version do not match")
	}
	if err := validateAddonAssetURL(info.AssetURL, parts[0], info.AssetName); err != nil {
		return err
	}
	return validateAddonAssetURL(info.ChecksumURL, parts[0], "SHA256SUMS.txt")
}

func addonExpectedChecksum(b []byte, name string) ([32]byte, error) {
	var sum [32]byte
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// GNU sha256sum text/binary form. Do not accept a substring filename,
		// comments, paths, or ambiguous multiple entries for the chosen asset.
		if len(line) < 67 || line[64] != ' ' || (line[65] != ' ' && line[65] != '*') {
			return sum, errors.New("invalid SHA256SUMS.txt format")
		}
		hash, err := hex.DecodeString(line[:64])
		if err != nil || len(hash) != len(sum) {
			return sum, errors.New("invalid SHA256 checksum")
		}
		file := line[66:]
		if err := addonSafeZipName(file); err != nil || strings.Contains(file, "/") {
			return sum, errors.New("unsafe filename in SHA256SUMS.txt")
		}
		if file != name {
			continue
		}
		if found {
			return sum, errors.New("duplicate addon SHA256 checksum")
		}
		copy(sum[:], hash)
		found = true
	}
	if !found {
		return sum, errors.New("SHA256SUMS.txt does not contain the exact addon asset filename")
	}
	return sum, nil
}

func (u *addonUpdater) requireStopped(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if u.running == nil {
		return errors.New("could not verify that WoW is closed; addon update refused")
	}
	running, err := u.running(ctx)
	if err != nil {
		return fmt.Errorf("could not verify that WoW is closed; addon update refused: %w", err)
	}
	if running {
		return errors.New("close World of Warcraft before installing or updating addons")
	}
	return ctx.Err()
}

// Reject symlinks/junctions in every ancestor as well as the target. The update
// paths are fixed; none of them are derived from archive or manifest paths.
func addonPlainDirectory(p string) error {
	for {
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing linked or non-directory update path: %s", p)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return nil
		}
		p = parent
	}
}

type addonInstalledState struct {
	exists  bool
	info    os.FileInfo
	toc     []byte
	version addonSemver
}

// InstalledCoachVersion reads the addon's own TOC, never the helper version or
// rules metadata. Missing/unreadable/invalid metadata reports 0.0.0 to the UI;
// ApplyAddonRelease separately refuses an existing but unidentifiable addon.
func InstalledCoachVersion(game string) string {
	if game == "" || !filepath.IsAbs(game) {
		return "0.0.0"
	}
	target := filepath.Join(filepath.Clean(game), "Interface", "AddOns", "ForeverCoach")
	if err := addonPlainDirectory(target); err != nil {
		return "0.0.0"
	}
	b, err := addonReadSmallFile(filepath.Join(target, "ForeverCoach.toc"))
	if err != nil {
		return "0.0.0"
	}
	version, err := addonExactTOCVersion(b)
	if err != nil {
		return "0.0.0"
	}
	return version.text
}

func addonExactTOCVersion(b []byte) (addonSemver, error) {
	value, found := "", false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "## Version:") {
			if found {
				return addonSemver{}, errors.New("duplicate addon TOC Version")
			}
			value, found = strings.TrimSpace(strings.TrimPrefix(line, "## Version:")), true
		}
	}
	return parseAddonVersion(value)
}

func addonInstalled(target string) (addonInstalledState, error) {
	var state addonInstalledState
	info, err := addonDirectoryIdentity(target)
	if err == nil && info == nil {
		state.version, _ = parseAddonVersion("0.0.0")
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := addonPlainDirectory(target); err != nil {
		return state, err
	}
	b, err := addonReadSmallFile(filepath.Join(target, "ForeverCoach.toc"))
	if err != nil {
		return state, fmt.Errorf("cannot determine installed addon version: %w", err)
	}
	v, err := addonExactTOCVersion(b)
	if err != nil {
		return state, fmt.Errorf("cannot safely compare installed addon version: %w", err)
	}
	return addonInstalledState{true, info, b, v}, nil
}

func (u *addonUpdater) apply(ctx context.Context, game string, release AddonReleaseInfo) (result AddonUpdateResult, err error) {
	if !addonInstallMutex.TryLock() {
		return result, errors.New("another addon update is in progress")
	}
	defer addonInstallMutex.Unlock()
	defer func() {
		if err != nil && errors.Is(err, os.ErrPermission) {
			err = fmt.Errorf("addon update needs write permission to the selected game folder; no other addon or SavedVariables were changed: %w", err)
		}
	}()
	if err := validateAddonRelease(release); err != nil {
		return result, err
	}
	if err := u.requireStopped(ctx); err != nil {
		return result, err
	}
	if game == "" || !filepath.IsAbs(game) {
		return result, errors.New("select an absolute World of Warcraft game folder before updating")
	}
	game = filepath.Clean(game)
	addons := filepath.Join(game, "Interface", "AddOns")
	// A fresh client may not have Interface\AddOns yet. The transaction creates
	// the missing folders only after every downloaded byte is verified, and
	// removes them again on rollback.
	if _, err := addonMissingDirs(game, addons); err != nil {
		return result, fmt.Errorf("invalid game addon folder: %w", err)
	}
	// Finish an install that was interrupted mid-swap before reading the
	// installed state, so the offer is checked against the real folders.
	interrupted := u.addonRecoverStaleLock(game)
	target := filepath.Join(addons, "ForeverCoach")
	before, err := addonInstalled(target)
	if err != nil {
		return result, err
	}
	remote, _ := parseAddonVersion(release.Version)
	if release.Repair {
		// Same version again only to complete an incomplete or interrupted
		// install; never a downgrade, never a plain reinstall.
		if remote.newerThan(before.version) || before.version.newerThan(remote) {
			return result, errors.New("the installed Relax Coach version changed; check again")
		}
		if !addonGuildRelease(release.Version) || (!interrupted && !addonRepairNeeded(game)) {
			return result, errors.New("the Relax Coach folders are complete; nothing to repair. Check again")
		}
	} else if !remote.newerThan(before.version) {
		return result, errors.New("no newer addon version to install; refusing to reinstall or downgrade")
	}

	trees, ancestors, err := addonSnapshot(game, release.Version)
	if err != nil {
		return result, err
	}
	checksums, err := u.fetch(ctx, release.ChecksumURL, u.limits.checksum, "release checksum")
	if err != nil {
		return result, err
	}
	want, err := addonExpectedChecksum(checksums, release.AssetName)
	if err != nil {
		return result, err
	}
	payload, err := u.fetch(ctx, release.AssetURL, u.limits.archive, "addon archive")
	if err != nil {
		return result, err
	}
	if got := sha256.Sum256(payload); got != want {
		return result, errors.New("addon SHA256 checksum mismatch; nothing was installed. Try again or contact the publisher")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return u.installTrees(ctx, game, release, payload, before, trees, ancestors)
}
