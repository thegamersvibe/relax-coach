package main

import "time"

// One Relax Coach window at a time (U23): a downloaded copy and the installed
// copy must not run together, and a second start brings the open window forward.
// The guard is a convenience, never a safety rule: any error lets the app run.

// nativeInstanceWait covers a copy that is just closing, such as the downloaded
// copy after "Open the installed Relax Coach" (it closes right after starting
// the installed one).
var nativeInstanceWait = 5 * time.Second

// nativeSingleInstance reports whether this GUI process may run. mode "start" is
// the new app of an app update: the old app has exited, and stopping here would
// fail its startup health and roll the update back, so it always runs (it still
// takes the lock when free). acquire waits up to the given time for the lock;
// focus brings the open window forward.
func nativeSingleInstance(mode string, acquire func(time.Duration) (bool, error), focus func()) bool {
	wait := nativeInstanceWait
	if mode == "start" {
		wait = 0
	}
	ok, err := acquire(wait)
	if err != nil || ok || mode == "start" {
		return true
	}
	focus()
	return false
}
