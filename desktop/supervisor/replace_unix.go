//go:build !windows

package supervisor

import "os"

func replaceFile(source, destination string) error { return os.Rename(source, destination) }
