package main

import (
	"os"
	"testing"
)

func TestFlightSoundNewVersionsRequireExactMedia(t *testing.T) {
	for _, version := range []string{"1.7.0", "1.7.0+reviewed", "1.7.1", "1.7.2", "1.8.0", "2.0.0"} {
		t.Run("accepted/"+version, func(t *testing.T) {
			result, err := campfireApply(t, version, funModeEntries(version))
			if err != nil || result.Version != version {
				t.Fatalf("flight bundle: %+v %v", result, err)
			}
		})
		for _, directory := range []bool{false, true} {
			name := "missing/"
			if directory {
				name = "directory/"
			}
			t.Run(name+version, func(t *testing.T) {
				entries := campfireEdit(funModeEntries(version), "Media/Reactions/flight.mp3")
				if directory {
					entries = append(entries, addonZipEntry{guildFixtureRoots[0] + "/Media/Reactions/flight.mp3/", "", os.ModeDir | 0755})
				}
				result, err := campfireApply(t, version, entries)
				if err == nil || result.Version != "" {
					t.Fatalf("incomplete flight bundle accepted: %+v %v", result, err)
				}
			})
		}
	}
}

func TestFlightSoundOlderVersionsKeepTheirOwnFileSet(t *testing.T) {
	for _, version := range []string{"1.4.3", "1.5.5", "1.5.6", "1.6.0", "1.6.1"} {
		t.Run("accepted/"+version, func(t *testing.T) {
			result, err := campfireApply(t, version, funModeEntries(version))
			if err != nil || result.Version != version {
				t.Fatalf("historical bundle: %+v %v", result, err)
			}
		})
		t.Run("refused/"+version, func(t *testing.T) {
			entries := campfireEdit(funModeEntries(version), "", "Media/Reactions/flight.mp3")
			result, err := campfireApply(t, version, entries)
			if err == nil || result.Version != "" {
				t.Fatalf("flight in older bundle: %+v %v", result, err)
			}
		})
	}
}
