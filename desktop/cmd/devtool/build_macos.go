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

func buildMacOS(ctx context.Context, root string, args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("build-macos must run on macOS")
	}
	arch := "arm64"
	dmg := false
	for _, a := range args {
		switch a {
		case "arm64", "amd64":
			arch = a
		case "--dmg":
			dmg = true
		default:
			return fmt.Errorf("unknown build-macos argument %q", a)
		}
	}
	for _, c := range []string{"otool", "install_name_tool", "codesign"} {
		if err := requireCommand(c); err != nil {
			return err
		}
	}
	desktop := filepath.Join(root, "desktop")
	bin := filepath.Join(desktop, "bin") // Wails3 convention: build artifacts live in bin/
	appName := "Lumilio Photos"
	app := filepath.Join(bin, appName+".app")
	macos := filepath.Join(app, "Contents", "MacOS")
	res := filepath.Join(app, "Contents", "Resources")
	fw := filepath.Join(app, "Contents", "Frameworks")
	exe := filepath.Join(macos, "lumilio-photos")
	version := envOr("LUMILIO_VERSION", "0.0.0")
	platformVersion := envOr("LUMILIO_PLATFORM_VERSION", strings.SplitN(version, "-", 2)[0])
	buildNumber := envOr("LUMILIO_BUILD_NUMBER", platformVersion)
	fmt.Println("==> Cleaning previous bundle")
	os.RemoveAll(app)
	for _, d := range []string{macos, res, fw} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
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
	fmt.Printf("==> Building Go binary (darwin-%s)\n", arch)
	ld := "-X server/internal/version.Version=" + version + " -X desktop/internal/buildinfo.Version=" + version + " -extldflags=-Wl,-rpath,@executable_path/../Frameworks"
	env := []string{"CGO_ENABLED=1", "GOOS=darwin", "GOARCH=" + arch, "CGO_LDFLAGS_ALLOW=-Xpreprocessor", "CGO_CFLAGS_ALLOW=-Xpreprocessor", "CGO_CFLAGS=-mmacosx-version-min=12.0", "CGO_LDFLAGS=-mmacosx-version-min=12.0", "MACOSX_DEPLOYMENT_TARGET=12.0"}
	// -trimpath -buildvcs=false mirror the Wails3 production BUILD_FLAGS.
	if err := runCmd(ctx, desktop, env, "go", "build", "-tags=sqlite_fts5", "-trimpath", "-buildvcs=false", "-ldflags", ld, "-o", exe, "."); err != nil {
		return err
	}
	if err := writeInfoPlist(filepath.Join(app, "Contents", "Info.plist"), platformVersion, buildNumber); err != nil {
		return err
	}
	icon := filepath.Join(desktop, "build", "darwin", "icons.icns")
	if !isFile(icon) {
		return fmt.Errorf("missing %s — run task desktop:generate:icons first", icon)
	}
	if err := copyFile(icon, filepath.Join(res, "AppIcon.icns")); err != nil {
		return err
	}
	// Mirror the Wails3 template's create:app:bundle: Assets.car (window
	// appearance assets) is copied when present; it is optional on dev runs.
	car := filepath.Join(desktop, "build", "darwin", "Assets.car")
	if isFile(car) {
		if err := copyFile(car, filepath.Join(res, "Assets.car")); err != nil {
			return err
		}
	}
	fmt.Println("==> Staging bundled runtime resources")
	stage := func(src, dst string) error {
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "    WARNING: missing %s — bundle will fall back to PATH at runtime\n", src)
				return nil
			}
			return err
		}
		return copyTree(src, dst)
	}
	r0 := filepath.Join(desktop, "resources")
	for _, p := range [][2]string{{filepath.Join(r0, "ffmpeg", "ffmpeg"), filepath.Join(res, "ffmpeg", "ffmpeg")}, {filepath.Join(r0, "ffmpeg", "ffprobe"), filepath.Join(res, "ffmpeg", "ffprobe")}, {filepath.Join(r0, "exiftool"), filepath.Join(res, "exiftool")}, {filepath.Join(desktop, "licenses"), filepath.Join(res, "licenses")}} {
		if err := stage(p[0], p[1]); err != nil {
			return err
		}
	}
	web := filepath.Join(root, "web", "dist")
	if os.Getenv("LUMILIO_WEB_DIST_PREBUILT") == "1" && isFile(filepath.Join(web, "index.html")) {
		fmt.Printf("    using prebuilt %s\n", web)
	} else {
		if !commandExists(executableName("vp")) {
			return fmt.Errorf("vp not found; run task setup before desktop-build")
		}
		fmt.Println("    building web frontend (vp build)")
		if err := runCmd(ctx, filepath.Join(root, "web"), nil, executableName("vp"), "build"); err != nil {
			return err
		}
	}
	if !isDir(web) {
		return fmt.Errorf("%s not found after vp build", web)
	}
	if err := copyTree(web, filepath.Join(res, "web")); err != nil {
		return err
	}
	if commandExists("dylibbundler") {
		fmt.Println("==> Bundling libvips dylib tree (dylibbundler)")
		if err := runCmd(ctx, root, nil, "dylibbundler", "-od", "-b", "-x", exe, "-d", fw+string(os.PathSeparator), "-p", "@executable_path/../Frameworks/"); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stderr, "    WARNING: dylibbundler not installed; skipping (brew install dylibbundler)")
	}
	moddir := findVipsModules(ctx)
	if moddir != "" {
		dst := filepath.Join(res, "lib", filepath.Base(moddir))
		os.MkdirAll(dst, 0755)
		for _, n := range []string{"vips-heif.dylib", "vips-magick.dylib"} {
			src := filepath.Join(moddir, n)
			if isFile(src) {
				if err := copyFile(src, filepath.Join(dst, n)); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(os.Stderr, "    WARNING: %s not found in %s\n", n, moddir)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "    WARNING: libvips modules not found")
	}
	if err := completeDylibClosure(ctx, exe, fw, filepath.Join(res, "lib")); err != nil {
		return err
	}
	if err := signMacBundle(ctx, app, exe, res, fw); err != nil {
		return err
	}
	fmt.Printf("==> Built: %s\n", app)
	if dmg {
		return createDMG(ctx, bin, desktop, appName, app, arch)
	}
	return nil
}

func writeInfoPlist(path, platformVersion, buildNumber string) error {
	s := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>Lumilio Photos</string><key>CFBundleDisplayName</key><string>Lumilio Photos</string>
<key>CFBundleExecutable</key><string>lumilio-photos</string><key>CFBundleIdentifier</key><string>com.edwinzhan.lumilio-photos</string>
<key>CFBundleIconFile</key><string>AppIcon</string><key>CFBundleIconName</key><string>appicon</string><key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>` + platformVersion + `</string><key>CFBundleVersion</key><string>` + buildNumber + `</string>
<key>LSMinimumSystemVersion</key><string>12.0</string><key>NSHighResolutionCapable</key><true/><key>LSUIElement</key><true/>
<key>NSLocalNetworkUsageDescription</key><string>Lumilio Photos discovers Lumen Intelligence servers on your local network via mDNS to enable optional AI features (semantic search, face recognition, OCR).</string>
<key>NSDesktopFolderUsageDescription</key><string>Lumilio Photos needs access to folders you select on the Desktop to store and manage your photo repositories.</string>
<key>NSDocumentsFolderUsageDescription</key><string>Lumilio Photos needs access to folders you select in Documents to store and manage your photo repositories.</string>
<key>NSDownloadsFolderUsageDescription</key><string>Lumilio Photos needs access to folders you select in Downloads to import or manage your photo repositories.</string>
<key>NSRemovableVolumesUsageDescription</key><string>Lumilio Photos needs ongoing access to external drives you select as Storage Locations for your photo repositories.</string>
<key>NSNetworkVolumesUsageDescription</key><string>Lumilio Photos needs ongoing access to network folders you select as Storage Locations for your photo repositories.</string>
<key>NSFileProviderDomainUsageDescription</key><string>Lumilio Photos needs access to folders you select in iCloud Drive or other file provider storage to manage your photo repositories.</string>
<key>NSBonjourServices</key><array><string>_lumen._tcp</string></array></dict></plist>
`
	return os.WriteFile(path, []byte(s), 0644)
}
func findVipsModules(ctx context.Context) string {
	for _, cmd := range [][]string{{"pkg-config", "--variable=prefix", "vips"}, {"brew", "--prefix", "vips"}} {
		if !commandExists(cmd[0]) {
			continue
		}
		b, e := outputCmd(ctx, "", nil, cmd[0], cmd[1:]...)
		if e != nil {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(strings.TrimSpace(string(b)), "lib", "vips-modules-*"))
		for _, m := range matches {
			if isDir(m) {
				return m
			}
		}
	}
	return ""
}
func macTargets(exe, fw, mods string) ([]string, error) {
	out := []string{exe}
	for _, root := range []string{fw, mods} {
		if !isDir(root) {
			continue
		}
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".dylib") {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func dylibRefs(ctx context.Context, f string) ([]string, error) {
	b, e := outputCmd(ctx, "", nil, "otool", "-L", f)
	if e != nil {
		return nil, e
	}
	s := bufio.NewScanner(strings.NewReader(string(b)))
	first := true
	var refs []string
	for s.Scan() {
		if first {
			first = false
			continue
		}
		fs := strings.Fields(s.Text())
		if len(fs) > 0 {
			refs = append(refs, fs[0])
		}
	}
	return refs, s.Err()
}

// deleteBundleRPaths removes duplicated LC_RPATH entries that dyld rejects
// with "duplicate LC_RPATH" when loading a bundled dylib. dylibbundler adds
// @executable_path/../Frameworks/ to every dylib it copies and the Homebrew
// build can already carry the same entry, so sweep both spellings until none
// remain. The executable itself keeps its single rpath for @rpath references.
func deleteBundleRPaths(ctx context.Context, f string) {
	for _, rpath := range []string{"@executable_path/../Frameworks/", "@executable_path/../Frameworks"} {
		for {
			out, err := outputCmd(ctx, "", nil, "otool", "-l", f)
			if err != nil || !bytes.Contains(out, []byte("path "+rpath+" ")) {
				break
			}
			if err := runCmd(ctx, "", nil, "install_name_tool", "-delete_rpath", rpath, f); err != nil {
				break
			}
		}
	}
}

func completeDylibClosure(ctx context.Context, exe, fw, mods string) error {
	prefixes := []string{"/opt/homebrew", "/usr/local"}
	if commandExists("brew") {
		if b, e := outputCmd(ctx, "", nil, "brew", "--prefix"); e == nil {
			prefixes = append([]string{strings.TrimSpace(string(b))}, prefixes...)
		}
	}
	find := func(base string) string {
		for _, p := range prefixes {
			for _, pat := range []string{filepath.Join(p, "lib", base), filepath.Join(p, "Cellar", "*", "*", "lib", base)} {
				m, _ := filepath.Glob(pat)
				for _, x := range m {
					if isFile(x) {
						return x
					}
				}
			}
		}
		return ""
	}
	for pass := 0; pass < 8; pass++ {
		changed := false
		targets, e := macTargets(exe, fw, mods)
		if e != nil {
			return e
		}
		for _, f := range targets {
			if strings.HasSuffix(f, ".dylib") {
				deleteBundleRPaths(ctx, f)
				_ = runCmd(ctx, "", nil, "install_name_tool", "-id", "@rpath/"+filepath.Base(f), f)
			}
			refs, e := dylibRefs(ctx, f)
			if e != nil {
				return e
			}
			for _, ref := range refs {
				base := filepath.Base(ref)
				bundle := filepath.Join(fw, base)
				eligible := strings.HasPrefix(ref, "/opt/homebrew/") || strings.HasPrefix(ref, "/usr/local/") || strings.HasPrefix(ref, "@rpath/") || strings.HasPrefix(ref, "@executable_path/../Frameworks/") || strings.HasPrefix(ref, "@loader_path/")
				if !eligible {
					continue
				}
				if !isFile(bundle) {
					src := find(base)
					if src != "" {
						fmt.Printf("    adding %s\n", base)
						if err := copyFile(src, bundle); err != nil {
							return err
						}
						os.Chmod(bundle, 0755)
						_ = runCmd(ctx, "", nil, "install_name_tool", "-id", "@rpath/"+base, bundle)
						changed = true
					}
				}
				if isFile(bundle) {
					newref := "@loader_path/" + base
					if f == exe {
						newref = "@rpath/" + base
					} else if strings.Contains(f, filepath.Join("Resources", "lib", "vips-modules-")) {
						newref = "@loader_path/../../../Frameworks/" + base
					}
					if ref != newref {
						_ = runCmd(ctx, "", nil, "install_name_tool", "-change", ref, newref, f)
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	targets, e := macTargets(exe, fw, mods)
	if e != nil {
		return e
	}
	for _, f := range targets {
		refs, _ := dylibRefs(ctx, f)
		for _, r := range refs {
			// OS-provided libraries (e.g. /System/Library/Frameworks/Cocoa)
			// are never bundled; verify only the reference kinds the closure
			// pass owns, mirroring the collection eligibility above.
			if strings.HasPrefix(r, "/System/Library/") || strings.HasPrefix(r, "/usr/lib/") {
				continue
			}
			if strings.HasPrefix(r, "/opt/homebrew/") || strings.HasPrefix(r, "/usr/local/") ||
				strings.HasPrefix(r, "@rpath/") || strings.HasPrefix(r, "@loader_path/") ||
				strings.HasPrefix(r, "@executable_path/") {
				base := filepath.Base(r)
				if !isFile(filepath.Join(fw, base)) {
					return fmt.Errorf("missing bundled dylib %s referenced by %s", base, f)
				}
			}
		}
	}
	return nil
}
func signMacBundle(ctx context.Context, app, exe, res, fw string) error {
	fmt.Println("==> Ad-hoc signing")
	targets, _ := macTargets(exe, fw, filepath.Join(res, "lib"))
	for _, f := range targets {
		if f == exe {
			continue
		}
		os.Chmod(f, 0755)
		_ = runCmd(ctx, "", nil, "codesign", "--force", "-s", "-", f)
	}
	filepath.WalkDir(res, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if st, se := os.Stat(p); se == nil && st.Mode()&0111 != 0 {
				_ = runCmd(ctx, "", nil, "codesign", "--force", "-s", "-", p)
			}
		}
		return nil
	})
	return runCmd(ctx, "", nil, "codesign", "--force", "--deep", "-s", "-", app)
}
func createDMG(ctx context.Context, bin, desktop, name, app, arch string) error {
	dmg := filepath.Join(bin, "Lumilio-Photos-"+arch+".dmg")
	src := filepath.Join(bin, "dmg-src")
	os.RemoveAll(dmg)
	os.RemoveAll(src)
	os.MkdirAll(src, 0755)
	if err := copyTree(app, filepath.Join(src, filepath.Base(app))); err != nil {
		return err
	}
	plain := func() error {
		_ = os.Symlink("/Applications", filepath.Join(src, "Applications"))
		return runCmd(ctx, "", nil, "hdiutil", "create", "-volname", name, "-srcfolder", src, "-ov", "-format", "UDZO", dmg)
	}
	if !commandExists("create-dmg") {
		return plain()
	}
	args := []string{"--volname", name, "--window-pos", "200", "120", "--window-size", "660", "400", "--icon-size", "120", "--icon", name + ".app", "165", "200", "--hide-extension", name + ".app", "--app-drop-link", "495", "200", "--no-internet-enable"}
	bg := filepath.Join(desktop, "build", "darwin", "background.png")
	if isFile(bg) {
		args = append(args, "--background", bg)
	}
	args = append(args, dmg, src)
	if err := runCmd(ctx, "", nil, "create-dmg", args...); err != nil {
		return plain()
	}
	return nil
}
