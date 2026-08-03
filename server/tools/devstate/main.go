package main

// Command dev-state owns the repository-local .local/dev lifecycle. It keeps
// the same marker and symlink protections as the former shell implementation,
// while using Go filesystem APIs so the command is usable on Windows too.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const markerMagic = "lumilio-dev-root-v1"

type environment struct {
	repositoryRoot string
	localRoot      string
	devRoot        string
	marker         string
	stateRoot      string
	storageRoot    string
}

func main() {
	if len(os.Args) != 3 {
		usage()
	}

	repositoryRoot, err := resolveRepositoryRoot(os.Args[2])
	if err != nil {
		fail(err)
	}
	env := newEnvironment(repositoryRoot)

	var runErr error
	switch os.Args[1] {
	case "init":
		runErr = env.initEnvironment()
	case "clean":
		runErr = env.cleanEnvironment()
	case "reset":
		runErr = env.resetEnvironment()
	case "purge":
		runErr = env.purgeEnvironment()
	default:
		usage()
	}
	if runErr != nil {
		fail(runErr)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <init|clean|reset|purge> <repository-root>\n", os.Args[0])
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "dev-state: %s\n", err)
	os.Exit(1)
}

func resolveRepositoryRoot(input string) (string, error) {
	absolute, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("resolve repository root %q: %w", input, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("repository root does not exist: %s", input)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("repository root does not exist: %s", input)
	}
	return filepath.Clean(resolved), nil
}

func newEnvironment(repositoryRoot string) environment {
	localRoot := filepath.Join(repositoryRoot, ".local")
	devRoot := filepath.Join(localRoot, "dev")
	return environment{
		repositoryRoot: repositoryRoot,
		localRoot:      localRoot,
		devRoot:        devRoot,
		marker:         filepath.Join(devRoot, ".lumilio-dev-root"),
		stateRoot:      filepath.Join(devRoot, "state"),
		storageRoot:    filepath.Join(devRoot, "storage"),
	}
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink path: %s", path)
	}
	return nil
}

func (env environment) assertRepository() error {
	if err := requireDirectory(filepath.Join(env.repositoryRoot, "server", "config")); err != nil {
		return fmt.Errorf("not a Lumilio Photos repository: %s", env.repositoryRoot)
	}
	if err := requireDirectory(filepath.Join(env.repositoryRoot, "web")); err != nil {
		return fmt.Errorf("not a Lumilio Photos repository: %s", env.repositoryRoot)
	}
	if err := requireFile(filepath.Join(env.repositoryRoot, "taskfile.yml")); err != nil {
		return fmt.Errorf("not a Lumilio Photos repository: %s", env.repositoryRoot)
	}
	if err := rejectSymlink(env.localRoot); err != nil {
		return err
	}
	return rejectSymlink(env.devRoot)
}

func requireDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return errMissingPath
	}
	return nil
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errMissingPath
	}
	return nil
}

var errMissingPath = errors.New("required path is missing")

func (env environment) verifyMarker() error {
	if err := rejectSymlink(env.marker); err != nil {
		return err
	}
	info, err := os.Stat(env.marker)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("missing regular development marker: %s", env.marker)
	}
	content, err := os.ReadFile(env.marker)
	if err != nil {
		return fmt.Errorf("read development marker %s: %w", env.marker, err)
	}
	if strings.TrimRight(string(content), "\n") != markerMagic {
		return fmt.Errorf("unexpected development marker content in %s", env.marker)
	}
	return nil
}

func (env environment) ensureRoot() error {
	if err := env.assertRepository(); err != nil {
		return err
	}
	if !pathExists(env.localRoot) {
		if err := os.Mkdir(env.localRoot, 0o700); err != nil {
			return fmt.Errorf("create development parent %s: %w", env.localRoot, err)
		}
	} else {
		info, err := os.Stat(env.localRoot)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("development parent is not a directory: %s", env.localRoot)
		}
	}
	if err := os.Chmod(env.localRoot, 0o700); err != nil {
		return fmt.Errorf("protect development parent %s: %w", env.localRoot, err)
	}

	if pathExists(env.devRoot) {
		info, err := os.Stat(env.devRoot)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("development root is not a directory: %s", env.devRoot)
		}
		return env.verifyMarker()
	}
	if err := os.Mkdir(env.devRoot, 0o700); err != nil {
		return fmt.Errorf("create development root %s: %w", env.devRoot, err)
	}
	if err := os.WriteFile(env.marker, []byte(markerMagic+"\n"), 0o600); err != nil {
		return fmt.Errorf("write development marker %s: %w", env.marker, err)
	}
	if err := os.Chmod(env.devRoot, 0o700); err != nil {
		return fmt.Errorf("protect development root %s: %w", env.devRoot, err)
	}
	return env.verifyMarker()
}

func (env environment) prepareRuntimeDirectories() error {
	directories := []string{
		filepath.Join(env.devRoot, "config"),
		env.stateRoot,
		env.storageRoot,
	}
	for _, directory := range directories {
		if err := rejectSymlink(directory); err != nil {
			return err
		}
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create runtime directory %s: %w", directory, err)
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return fmt.Errorf("protect runtime directory %s: %w", directory, err)
		}
	}
	return nil
}

func (env environment) assertServerStopped() error {
	curl, err := exec.LookPath("curl")
	if err != nil {
		return errors.New("curl is required to prove the development server is stopped")
	}
	cmd := exec.Command(
		curl,
		"--noproxy", "*",
		"--silent",
		"--max-time", "1",
		"--output", os.DevNull,
		"http://127.0.0.1:6680/api/v1/health/ready",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		return errors.New("the development server is still listening on 127.0.0.1:6680")
	}
	return nil
}

func describeTarget(target string) string {
	if !pathExists(target) {
		return ""
	}
	size, err := directorySize(target)
	if err != nil {
		return fmt.Sprintf("  %s (unknown size)", target)
	}
	return fmt.Sprintf("  %s (%s)", target, humanSize(size))
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, name := range units {
		value /= unit
		if value < unit || name == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%d B", bytes)
}

func (env environment) removeDevChild(target string) error {
	relative, err := filepath.Rel(env.devRoot, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing path outside development root: %s", target)
	}
	if err := rejectSymlink(target); err != nil {
		return err
	}
	if !pathExists(target) {
		return nil
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove %s: %w", target, err)
	}
	return nil
}

func (env environment) initEnvironment() error {
	if err := env.ensureRoot(); err != nil {
		return err
	}
	if err := env.prepareRuntimeDirectories(); err != nil {
		return err
	}
	fmt.Printf("Development root: %s\n", env.devRoot)
	return nil
}

func (env environment) cleanEnvironment() error {
	if err := env.ensureRoot(); err != nil {
		return err
	}
	if err := env.assertServerStopped(); err != nil {
		return err
	}
	stateRoot := env.stateRoot
	if err := rejectSymlink(stateRoot); err != nil {
		return err
	}
	fmt.Println("Removing rebuildable development outputs:")
	if description := describeTarget(filepath.Join(stateRoot, "indexes")); description != "" {
		fmt.Println(description)
	}
	if description := describeTarget(filepath.Join(stateRoot, "logs")); description != "" {
		fmt.Println(description)
	}
	if err := env.removeDevChild(filepath.Join(stateRoot, "indexes")); err != nil {
		return err
	}
	return env.removeDevChild(filepath.Join(stateRoot, "logs"))
}

func (env environment) resetEnvironment() error {
	if err := env.ensureRoot(); err != nil {
		return err
	}
	if err := env.assertServerStopped(); err != nil {
		return err
	}
	fmt.Println("Removing development application state; media storage is preserved:")
	if description := describeTarget(env.stateRoot); description != "" {
		fmt.Println(description)
	}
	return env.removeDevChild(env.stateRoot)
}

func (env environment) purgeEnvironment() error {
	if err := env.assertRepository(); err != nil {
		return err
	}
	if !pathExists(env.devRoot) {
		fmt.Printf("No unified development environment exists at %s\n", env.devRoot)
		return nil
	}
	if err := env.verifyMarker(); err != nil {
		return err
	}
	if err := env.assertServerStopped(); err != nil {
		return err
	}
	if os.Getenv("CONFIRM_DEV_PURGE") != "dev-purge" {
		return errors.New("set CONFIRM_DEV_PURGE=dev-purge when invoking task dev-purge")
	}
	fmt.Println("Removing the complete development environment, including media:")
	if description := describeTarget(env.devRoot); description != "" {
		fmt.Println(description)
	}
	if err := rejectSymlink(env.stateRoot); err != nil {
		return err
	}
	if err := rejectSymlink(env.storageRoot); err != nil {
		return err
	}
	if err := os.RemoveAll(env.devRoot); err != nil {
		return fmt.Errorf("remove %s: %w", env.devRoot, err)
	}
	_ = os.Remove(env.localRoot)
	return nil
}
