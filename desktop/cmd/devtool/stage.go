package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// goTags are the build tags of every Desktop binary: the embedded Server
// needs SQLite's FTS5.
const goTags = "-tags=sqlite_fts5"

// macResourceLibDir is where the packaged app keeps its native libraries,
// relative to the executable. The link step records it as the one rpath.
const macResourceLibDir = "@executable_path/../Resources/lib"

// nativeStage is what staging the native libraries leaves for the build.
type nativeStage struct {
	// cgoLDFlags make the linker resolve libraries from the staged copies
	// and record where the packaged app finds them.
	cgoLDFlags []string
}

// stageResources fills desktop/resources with everything the packaged app
// ships beside its executable, so that `mygo build` copies and signs it
// with the app: the Web SPA, the license notices, and the native libraries
// of the current platform. The media tools are put there by fetch-resources.
//
// Native libraries are staged before the app is linked, not patched into it
// afterwards: the packager signs the bundle and writes the update archive in
// one step, and a bundle changed after that would match neither.
func stageResources(ctx context.Context, root, arch string) (nativeStage, error) {
	desktop := filepath.Join(root, "desktop")
	resources := filepath.Join(desktop, "resources")

	fmt.Println("==> Staging the Web SPA")
	web := filepath.Join(root, "web", "dist")
	if os.Getenv("LUMILIO_WEB_DIST_PREBUILT") == "1" && isFile(filepath.Join(web, "index.html")) {
		fmt.Printf("    using prebuilt %s\n", web)
	} else {
		if !commandExists(executableName("vp")) {
			return nativeStage{}, fmt.Errorf("vp not found; run task setup, or build web/dist and set LUMILIO_WEB_DIST_PREBUILT=1")
		}
		if err := runCmd(ctx, filepath.Join(root, "web"), nil, executableName("vp"), "build"); err != nil {
			return nativeStage{}, err
		}
	}
	if !isFile(filepath.Join(web, "index.html")) {
		return nativeStage{}, fmt.Errorf("%s has no index.html after the Web build", web)
	}
	if err := replaceTree(web, filepath.Join(resources, "web")); err != nil {
		return nativeStage{}, err
	}

	fmt.Println("==> Staging license notices")
	if err := replaceTree(filepath.Join(desktop, "licenses"), filepath.Join(resources, "licenses")); err != nil {
		return nativeStage{}, err
	}

	for _, tool := range []string{"ffmpeg", "exiftool"} {
		if !isDir(filepath.Join(resources, tool)) {
			fmt.Fprintf(os.Stderr, "    WARNING: resources/%s is missing — run `go run ./cmd/devtool fetch-resources`; the app would fall back to PATH\n", tool)
		}
	}

	switch runtime.GOOS {
	case "darwin":
		return stageMacLibraries(ctx, desktop, arch)
	case "windows":
		return nativeStage{}, stageWindowsLibraries(ctx, desktop, arch)
	default:
		return nativeStage{}, fmt.Errorf("the Desktop App is packaged on macOS and Windows, not %s", runtime.GOOS)
	}
}

func replaceTree(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return copyTree(src, dst)
}

// probeBuild links the app once without packaging it, to learn which native
// libraries it needs. It shares the compile cache with the packaged build.
func probeBuild(ctx context.Context, desktop, arch, output string) error {
	fmt.Println("==> Linking a probe binary to resolve native libraries")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return runCmd(ctx, desktop, cgoEnv(arch), "go", "build", goTags, "-trimpath", "-buildvcs=false", "-o", output, ".")
}

func cgoEnv(arch string) []string {
	env := []string{"CGO_ENABLED=1", "GOARCH=" + arch, "CGO_CFLAGS_ALLOW=-Xpreprocessor", "CGO_LDFLAGS_ALLOW=-Xpreprocessor|-Wl,-rpath,@executable_path/.*"}
	if runtime.GOOS == "darwin" {
		env = append(env, "CGO_CFLAGS=-mmacosx-version-min=12.0", "MACOSX_DEPLOYMENT_TARGET=12.0")
	}
	return env
}

// stageMacLibraries collects the dylib closure of the app and of the libvips
// modules into resources/darwin-<arch>/lib, relocatable: every library is
// named and referenced as @rpath/<name>, and the executable's single rpath
// points at the bundle's copy. It also leaves a directory of the unversioned
// names the linker looks for, so the packaged build links against the staged
// copies and records @rpath references from the start.
func stageMacLibraries(ctx context.Context, desktop, arch string) (nativeStage, error) {
	for _, tool := range []string{"dylibbundler", "otool", "install_name_tool", "codesign"} {
		if err := requireCommand(tool); err != nil {
			return nativeStage{}, fmt.Errorf("%w (brew install dylibbundler; the others come with Xcode's command line tools)", err)
		}
	}
	platformDir := filepath.Join(desktop, "resources", "darwin-"+arch)
	lib := filepath.Join(platformDir, "lib")
	work := filepath.Join(desktop, "bin", "native")
	link := filepath.Join(work, "link")
	probe := filepath.Join(work, "probe")
	for _, dir := range []string{platformDir, work} {
		if err := os.RemoveAll(dir); err != nil {
			return nativeStage{}, err
		}
	}
	for _, dir := range []string{lib, link} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nativeStage{}, err
		}
	}
	if err := probeBuild(ctx, desktop, arch, probe); err != nil {
		return nativeStage{}, err
	}

	// The names the linker asks for (libvips.dylib) are symlinks beside the
	// versioned library the app loads (libvips.42.dylib). Record them before
	// the probe's references are rewritten.
	direct, err := dylibRefs(ctx, probe)
	if err != nil {
		return nativeStage{}, err
	}
	linkNames := map[string]string{}
	for _, ref := range direct {
		if !strings.HasPrefix(ref, "/opt/homebrew/") && !strings.HasPrefix(ref, "/usr/local/") {
			continue
		}
		target, err := filepath.EvalSymlinks(ref)
		if err != nil {
			return nativeStage{}, fmt.Errorf("resolve %s: %w", ref, err)
		}
		siblings, _ := filepath.Glob(filepath.Join(filepath.Dir(ref), "*.dylib"))
		for _, sibling := range siblings {
			if resolved, err := filepath.EvalSymlinks(sibling); err == nil && resolved == target {
				linkNames[filepath.Base(sibling)] = filepath.Base(ref)
			}
		}
	}

	fmt.Println("==> Collecting the dylib closure")
	if err := runCmd(ctx, desktop, nil, "dylibbundler", "-od", "-b", "-x", probe, "-d", lib+string(os.PathSeparator), "-p", "@rpath/"); err != nil {
		return nativeStage{}, err
	}
	if modules := findVipsModules(ctx); modules == "" {
		fmt.Fprintln(os.Stderr, "    WARNING: libvips modules not found; HEIF and ImageMagick formats would be unavailable")
	} else {
		staged := filepath.Join(lib, filepath.Base(modules))
		if err := os.MkdirAll(staged, 0o755); err != nil {
			return nativeStage{}, err
		}
		for _, name := range []string{"vips-heif.dylib", "vips-magick.dylib"} {
			src := filepath.Join(modules, name)
			if !isFile(src) {
				fmt.Fprintf(os.Stderr, "    WARNING: %s not found in %s\n", name, modules)
				continue
			}
			module := filepath.Join(staged, name)
			if err := copyFile(src, module); err != nil {
				return nativeStage{}, err
			}
			if err := os.Chmod(module, 0o755); err != nil {
				return nativeStage{}, err
			}
			// A module brings libraries the app itself does not link.
			if err := runCmd(ctx, desktop, nil, "dylibbundler", "-of", "-b", "-x", module, "-d", lib+string(os.PathSeparator), "-p", "@rpath/"); err != nil {
				return nativeStage{}, err
			}
		}
	}

	libraries, err := macLibraries(lib)
	if err != nil {
		return nativeStage{}, err
	}
	fmt.Printf("==> Relocating and signing %d libraries\n", len(libraries))
	for _, file := range libraries {
		if err := os.Chmod(file, 0o755); err != nil {
			return nativeStage{}, err
		}
		// dylibbundler gives each copy an rpath of its own, which dyld
		// rejects as a duplicate of the executable's.
		if err := deleteRPaths(ctx, file); err != nil {
			return nativeStage{}, err
		}
		if filepath.Dir(file) == lib {
			if err := runCmd(ctx, "", nil, "install_name_tool", "-id", "@rpath/"+filepath.Base(file), file); err != nil {
				return nativeStage{}, err
			}
		}
	}
	for _, file := range libraries {
		if err := verifyRelocated(ctx, file, lib); err != nil {
			return nativeStage{}, err
		}
		// Changing a Mach-O invalidates its signature, and Apple silicon
		// runs no unsigned code. The packager signs only what has none.
		if err := runCmd(ctx, "", nil, "codesign", "--force", "-s", "-", file); err != nil {
			return nativeStage{}, err
		}
	}

	for name, target := range linkNames {
		if !isFile(filepath.Join(lib, target)) {
			return nativeStage{}, fmt.Errorf("the closure does not hold %s, which the app links", target)
		}
		if err := os.Symlink(filepath.Join(lib, target), filepath.Join(link, name)); err != nil {
			return nativeStage{}, err
		}
	}
	return nativeStage{cgoLDFlags: []string{"-L" + link, "-Wl,-rpath," + macResourceLibDir, "-mmacosx-version-min=12.0"}}, nil
}

func macLibraries(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".dylib") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func deleteRPaths(ctx context.Context, file string) error {
	for {
		out, err := outputCmd(ctx, "", nil, "otool", "-l", file)
		if err != nil {
			return err
		}
		rpath := firstRPath(out)
		if rpath == "" {
			return nil
		}
		if err := runCmd(ctx, "", nil, "install_name_tool", "-delete_rpath", rpath, file); err != nil {
			return err
		}
	}
}

// firstRPath reads the path of the first LC_RPATH load command in the output
// of `otool -l`.
func firstRPath(loadCommands []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(loadCommands))
	inRPath := false
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		switch {
		case len(fields) == 2 && fields[0] == "cmd":
			inRPath = fields[1] == "LC_RPATH"
		case inRPath && len(fields) >= 2 && fields[0] == "path":
			return fields[1]
		}
	}
	return ""
}

// verifyRelocated fails when a staged library still refers to a library
// outside the bundle, which would load from the build machine's Homebrew on
// the developer's Mac and fail on everyone else's.
func verifyRelocated(ctx context.Context, file, lib string) error {
	refs, err := dylibRefs(ctx, file)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if strings.HasPrefix(ref, "/System/Library/") || strings.HasPrefix(ref, "/usr/lib/") {
			continue
		}
		name, bundled := strings.CutPrefix(ref, "@rpath/")
		if !bundled || !isFile(filepath.Join(lib, name)) {
			return fmt.Errorf("%s refers to %s, which is not in the bundle", filepath.Base(file), ref)
		}
	}
	return nil
}

func findVipsModules(ctx context.Context) string {
	for _, cmd := range [][]string{{"pkg-config", "--variable=prefix", "vips"}, {"brew", "--prefix", "vips"}} {
		if !commandExists(cmd[0]) {
			continue
		}
		out, err := outputCmd(ctx, "", nil, cmd[0], cmd[1:]...)
		if err != nil {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(strings.TrimSpace(string(out)), "lib", "vips-modules-*"))
		for _, match := range matches {
			if isDir(match) {
				return match
			}
		}
	}
	return ""
}

// dylibRefs lists the libraries a Mach-O file loads. For a library the list
// starts with its own install name, which is skipped with the header line.
func dylibRefs(ctx context.Context, file string) ([]string, error) {
	out, err := outputCmd(ctx, "", nil, "otool", "-L", file)
	if err != nil {
		return nil, err
	}
	own := ""
	if strings.HasSuffix(file, ".dylib") {
		if id, err := outputCmd(ctx, "", nil, "otool", "-D", file); err == nil {
			if lines := strings.Split(strings.TrimSpace(string(id)), "\n"); len(lines) > 1 {
				own = strings.TrimSpace(lines[len(lines)-1])
			}
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	first := true
	var refs []string
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		if fields := strings.Fields(scanner.Text()); len(fields) > 0 && fields[0] != own {
			refs = append(refs, fields[0])
		}
	}
	return refs, scanner.Err()
}

// stageWindowsLibraries collects the mingw64 DLL closure of the app and of
// the libvips modules into resources/windows-<arch>, which the packager
// installs beside the executable, where Windows looks for DLLs first.
func stageWindowsLibraries(ctx context.Context, desktop, arch string) error {
	if err := requireCommand("ntldd"); err != nil {
		return fmt.Errorf("packaging on Windows must run in MSYS2 MINGW64: %w", err)
	}
	platformDir := filepath.Join(desktop, "resources", "windows-"+arch)
	work := filepath.Join(desktop, "bin", "native")
	probe := filepath.Join(work, "probe.exe")
	for _, dir := range []string{platformDir, work} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(platformDir, 0o755); err != nil {
		return err
	}
	if err := probeBuild(ctx, desktop, arch, probe); err != nil {
		return err
	}
	fmt.Println("==> Collecting the mingw64 DLL closure")
	if err := collectWindowsDLLs(ctx, probe, platformDir); err != nil {
		return err
	}
	modules, _ := filepath.Glob("/mingw64/lib/vips-modules-*")
	if len(modules) == 0 {
		fmt.Fprintln(os.Stderr, "    WARNING: libvips modules not found; HEIF and ImageMagick formats would be unavailable")
		return nil
	}
	module := modules[0]
	fmt.Printf("==> Staging libvips modules (%s)\n", module)
	if err := copyTree(module, filepath.Join(platformDir, "lib", filepath.Base(module))); err != nil {
		return err
	}
	dlls, _ := filepath.Glob(filepath.Join(module, "*.dll"))
	for _, dll := range dlls {
		if err := collectWindowsDLLs(ctx, dll, platformDir); err != nil {
			return err
		}
	}
	return nil
}

func collectWindowsDLLs(ctx context.Context, binary, dest string) error {
	out, err := outputCmd(ctx, "", nil, "ntldd", "-R", binary)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	seen := map[string]bool{}
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(strings.ToLower(line), "mingw64") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		path := fields[2]
		if seen[path] {
			continue
		}
		seen[path] = true
		windowsPath, err := outputCmd(ctx, "", nil, "cygpath", "-w", path)
		if err != nil {
			continue
		}
		src := strings.TrimSpace(string(windowsPath))
		dst := filepath.Join(dest, filepath.Base(src))
		if isFile(dst) {
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return scanner.Err()
}
