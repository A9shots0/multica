//go:build !windows

package main

import "os"

func replaceHistoryCheckpoint(source, destination string) error {
	return os.Rename(source, destination)
}
