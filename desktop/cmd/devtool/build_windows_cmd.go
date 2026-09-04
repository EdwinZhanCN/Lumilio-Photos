package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func buildWindows(ctx context.Context, root string, args []string) error {
	if err := requireCommand("ntldd"); err != nil {
		return fmt.Errorf("build-windows must run in MSYS2 MINGW64: %w", err)
	}
	desktop := filepath.Join(root, "desktop")
	resources := filepath.Join(desktop, "resources")
	// Wails3 convention: build artifacts live in bin/; build/ stays config-only.
	app := filepath.Join(desktop, "bin", "windows", "Lumilio Photos")
	exe := filepath.Join(app, "lumilio-photos.exe")
	version := envOr("LUMILIO_VERSION", "0.0.0")
	platformVersion := envOr("LUMILIO_PLATFORM_VERSION", strings.SplitN(version, "-", 2)[0])
	fmt.Println("==> Cleaning previous build")
	os.RemoveAll(app)
	if err := os.MkdirAll(app, 0755); err != nil {
		return err
	}
	frontend := filepath.Join(desktop, "frontend", "dist")
	fmt.Println("==> Ensuring control panel bundle")
	if os.Getenv("LUMILIO_FRONTEND_DIST_PREBUILT") == "1" && isFile(filepath.Join(frontend, "index.html")) {
		fmt.Printf("    using prebuilt %s\n", frontend)
	} else {
		if !commandExists(executableName("vp")) {
			return fmt.Errorf("vp not found; the Go binary embeds frontend/dist")
		}
		if err := runCmd(ctx, filepath.Join(desktop, "frontend"), []string{"CI=1", "VITE_GIT_HOOKS=0"}, executableName("vp"), "install"); err != nil {
			return err
		}
		if err := runCmd(ctx, filepath.Join(desktop, "frontend"), []string{"CI=1", "VITE_GIT_HOOKS=0"}, executableName("vp"), "run", "build"); err != nil {
			return err
		}
	}
	fmt.Println("==> Building Go binary (windows/amd64, CGo via mingw64)")
	ld := "-X server/internal/version.Version=" + version + " -X desktop/internal/buildinfo.Version=" + version + " -H windowsgui"
	// Windows icon/version resources: wails3 generate syso compiles
	// build/windows/icon.ico + manifest + info.json into a .syso that go build
	// links into the executable, mirroring the Wails3 template's windows build
	// flow. The file must live in the module package directory (desktop/).
	syso := filepath.Join(desktop, "wails_windows_amd64.syso")
	os.Remove(syso)
	if commandExists("wails3") {
		fmt.Println("==> Generating Windows .syso resources")
		versionInfo, err := writeWindowsVersionInfo(filepath.Join(desktop, "bin"), platformVersion, version)
		if err != nil {
			return err
		}
		defer os.Remove(versionInfo)
		if err := runCmd(ctx, desktop, nil, "wails3", "generate", "syso",
			"-arch", "amd64",
			"-icon", filepath.Join(desktop, "build", "windows", "icon.ico"),
			"-manifest", filepath.Join(desktop, "build", "windows", "wails.exe.manifest"),
			"-info", versionInfo,
			"-out", syso); err != nil {
			return err
		}
		defer os.Remove(syso)
	} else {
		fmt.Fprintln(os.Stderr, "    WARNING: wails3 CLI not found; Windows exe will use default resources (go install github.com/wailsapp/wails/v3/cmd/wails3)")
	}
	// -trimpath -buildvcs=false mirror the Wails3 production BUILD_FLAGS.
	if err := runCmd(ctx, desktop, []string{"CGO_ENABLED=1"}, "go", "build", "-tags=sqlite_fts5", "-trimpath", "-buildvcs=false", "-ldflags", ld, "-o", exe, "."); err != nil {
		return err
	}
	fmt.Println("==> Collecting mingw64 DLL closure")
	if err := collectWindowsDLLs(ctx, exe, app); err != nil {
		return err
	}
	fmt.Println("==> Staging bundled runtime resources")
	resOut := filepath.Join(app, "resources")
	os.MkdirAll(resOut, 0755)
	stageWarn := func(src, dst string) error {
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "    WARNING: missing %s — bundle will fall back to PATH at runtime\n", src)
				return nil
			}
			return err
		}
		return copyTree(src, dst)
	}
	if err := stageWarn(filepath.Join(resources, "ffmpeg"), filepath.Join(resOut, "ffmpeg")); err != nil {
		return err
	}
	if err := stageWarn(filepath.Join(resources, "exiftool"), filepath.Join(resOut, "exiftool")); err != nil {
		return err
	}
	if err := stageWarn(filepath.Join(desktop, "licenses"), filepath.Join(resOut, "licenses")); err != nil {
		return err
	}
	web := filepath.Join(root, "web", "dist")
	if isFile(filepath.Join(web, "index.html")) {
		fmt.Println("==> Staging web SPA")
		if err := copyTree(web, filepath.Join(resOut, "web")); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(os.Stderr, "    WARNING: %s missing — app will run API-only\n", web)
	}
	mods, _ := filepath.Glob("/mingw64/lib/vips-modules-*")
	if len(mods) > 0 {
		mod := mods[0]
		fmt.Printf("==> Staging libvips modules (%s)\n", mod)
		dst := filepath.Join(resOut, "lib", filepath.Base(mod))
		if err := copyTree(mod, dst); err != nil {
			return err
		}
		dlls, _ := filepath.Glob(filepath.Join(mod, "*.dll"))
		for _, d := range dlls {
			if err := collectWindowsDLLs(ctx, d, app); err != nil {
				return err
			}
		}
	}
	fmt.Printf("==> Built: %s\n", app)
	return nil
}

func writeWindowsVersionInfo(directory, platformVersion, productVersion string) (string, error) {
	payload := map[string]any{
		"fixed": map[string]string{"file_version": platformVersion},
		"info": map[string]any{
			"0000": map[string]string{
				"ProductVersion":  productVersion,
				"CompanyName":     "Lumilio Photos",
				"FileDescription": "Local-first photo management",
				"LegalCopyright":  "(c) Lumilio Photos",
				"ProductName":     "Lumilio Photos",
				"Comments":        "Local-first photo management",
			},
		},
	}
	data, err := json.MarshalIndent(payload, "", "\t")
	if err != nil {
		return "", fmt.Errorf("encode Windows version info: %w", err)
	}
	file, err := os.CreateTemp(directory, ".lumilio-windows-version-*.json")
	if err != nil {
		return "", fmt.Errorf("create Windows version info: %w", err)
	}
	name := file.Name()
	if _, err = file.Write(append(data, '\n')); err != nil {
		file.Close()
		os.Remove(name)
		return "", fmt.Errorf("write Windows version info: %w", err)
	}
	if err = file.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("close Windows version info: %w", err)
	}
	return name, nil
}
func collectWindowsDLLs(ctx context.Context, binary, dest string) error {
	out, err := outputCmd(ctx, "", nil, "ntldd", "-R", binary)
	if err != nil {
		return err
	}
	s := bufio.NewScanner(strings.NewReader(string(out)))
	seen := map[string]bool{}
	for s.Scan() {
		line := s.Text()
		if !strings.Contains(strings.ToLower(line), "mingw64") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		p := fields[2]
		if seen[p] {
			continue
		}
		seen[p] = true
		u, err := outputCmd(ctx, "", nil, "cygpath", "-w", p)
		if err != nil {
			continue
		}
		src := strings.TrimSpace(string(u))
		dst := filepath.Join(dest, filepath.Base(src))
		if isFile(dst) {
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return s.Err()
}
