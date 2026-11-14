//go:build !windows

package main

import "time"

func nativeAcquireInstance(time.Duration) (bool, error) { return true, nil }
func nativeFocusOther()                                 {}
