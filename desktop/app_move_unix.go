//go:build !windows && !linux

package main

import "os"

// appMoveNew: no-clobber publication for other test platforms (hard link,
// then drop the old name). Windows and Linux move in one step.
func appMoveNew(from, to string) error {
	if err := os.Link(from, to); err != nil {
		return err
	}
	return os.Remove(from)
}
