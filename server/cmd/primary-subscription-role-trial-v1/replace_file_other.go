//go:build !windows

package main

import "os"

func replaceExistingFile(source, destination string) error {
	return os.Rename(source, destination)
}
