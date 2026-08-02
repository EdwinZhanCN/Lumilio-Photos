package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"desktop/internal/resources"
)

func main() {
	root := flag.String("root", "resources/payload", "resource payload root")
	output := flag.String("output", "resources/manifest.json", "manifest output path")
	version := flag.String("version", "0.1.0", "payload version")
	flag.Parse()

	entries := make([]resources.Entry, 0)
	err := filepath.WalkDir(*root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == ".gitkeep" {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("resource payload may not contain symlinks: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		relative, err := filepath.Rel(*root, path)
		if err != nil {
			return err
		}
		entries = append(entries, resources.Entry{
			LogicalName: filepath.ToSlash(relative), Platform: runtime.GOOS, Arch: runtime.GOARCH,
			Version: *version, SHA256: hex.EncodeToString(hash[:]), Mode: uint32(info.Mode().Perm()), Path: filepath.ToSlash(relative),
		})
		return nil
	})
	if err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(resources.Manifest{SchemaVersion: resources.SchemaVersion, Entries: entries}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0o600); err != nil {
		panic(err)
	}
}
