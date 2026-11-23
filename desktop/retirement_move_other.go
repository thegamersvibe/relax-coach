//go:build !windows && !linux

package main

import "fmt"

func retirementPlatformMoveNoReplace(oldAbs, newAbs string) error {
	return fmt.Errorf("atomic no-replace move unavailable; source left in place")
}
