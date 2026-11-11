//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

func appSingleLink(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return errors.New("refusing hard-linked app update file")
	}
	return nil
}
func appRejectReparse(path string) error    { return nil }
func appPrivateDirectory(path string) error { return os.Mkdir(path, 0700) }

