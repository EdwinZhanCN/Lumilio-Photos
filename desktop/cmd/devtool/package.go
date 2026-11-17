package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// packageApp stages the resources and runs `mygo build`, which compiles the
// app, bundles and signs it, and writes the installer and, for a build with
// an update key, the signed update archive and manifest.
//
// The release identity comes from the environment, as in the release
// workflow: LUMILIO_VERSION is the product version the app reports and the
// updater compares. mygo.config.ts reads the same variables for the bundle.
func packageApp(ctx context.Context, root string, args []string) error {
	arch := runtime.GOARCH
	installer, stageOnly := false, false
	for _, arg := range args {
		switch arg {
		case "arm64", "amd64":
			arch = arg
		case "--installer":
			installer = true
		case "--stage-only":
			stageOnly = true
		default:
			return fmt.Errorf("unknown package argument %q", arg)
		}
	}
	desktop := filepath.Join(root, "desktop")
	stage, err := stageResources(ctx, root, arch)
	if err != nil {
		return err
	}
	if stageOnly {
		return nil
	}

	version := envOr("LUMILIO_VERSION", "0.0.0")
	env := append(cgoEnv(arch),
		"GOFLAGS=-buildvcs=false "+goTags,
		"LUMILIO_VERSION="+version,
		// The Server and the Desktop report the same version as the bundle.
		"MYGO_LDFLAGS=-X server/internal/foundation/version.Version="+version+" -X desktop/internal/buildinfo.Version="+version,
	)
	if len(stage.cgoLDFlags) > 0 {
		env = append(env, "CGO_LDFLAGS="+strings.Join(stage.cgoLDFlags, " "))
	}
	build := []string{"tool", "mygo", "build", "-platform", runtime.GOOS + "/" + arch}
	if !installer {
		// A plain package is the app alone: no disk image, no setup program.
		build = append(build, "-skip-dmg", "-skip-installer")
	}
	fmt.Printf("==> Packaging Lumilio Photos %s (%s/%s)\n", version, runtime.GOOS, arch)
	if err := runCmd(ctx, desktop, env, "go", build...); err != nil {
		return err
	}
	out := filepath.Join(desktop, "bin", runtime.GOOS+"-"+arch)
	if runtime.GOOS == "darwin" {
		if err := verifyMacBundle(ctx, filepath.Join(out, "Lumilio Photos.app")); err != nil {
			return err
		}
	}
	fmt.Printf("==> Packaged: %s\n", out)
	return nil
}

// verifyMacBundle checks what packaging must not get wrong silently: the
// executable loads its libraries from the bundle alone, and the signature
// covers everything that was staged.
func verifyMacBundle(ctx context.Context, app string) error {
	exe := filepath.Join(app, "Contents", "MacOS", "Lumilio Photos")
	lib := filepath.Join(app, "Contents", "Resources", "lib")
	if !isFile(exe) {
		return fmt.Errorf("the packaged app has no executable at %s", exe)
	}
	if err := verifyRelocated(ctx, exe, lib); err != nil {
		return fmt.Errorf("the packaged executable is not self-contained: %w", err)
	}
	loadCommands, err := outputCmd(ctx, "", nil, "otool", "-l", exe)
	if err != nil {
		return err
	}
	if rpath := firstRPath(loadCommands); rpath != macResourceLibDir {
		return fmt.Errorf("the packaged executable looks for libraries in %q, want %q", rpath, macResourceLibDir)
	}
	if err := runCmd(ctx, "", nil, "codesign", "--verify", "--deep", "--strict", app); err != nil {
		return fmt.Errorf("the packaged app's signature does not verify: %w", err)
	}
	return nil
}
