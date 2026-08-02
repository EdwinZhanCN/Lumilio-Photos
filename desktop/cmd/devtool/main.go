package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "interrupted")
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage()
		return nil
	}
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	switch args[0] {
	case "generate-third-party-notices", "notices":
		return generateThirdPartyNotices(ctx, root)
	case "fetch-resources":
		return fetchResources(ctx, root, args[1:])
	case "build-windows":
		return buildWindows(ctx, root, args[1:])
	case "build-macos":
		return buildMacOS(ctx, root, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  go run ./cmd/devtool <command> [options]

Commands:
  notices                       Generate desktop/licenses/THIRD_PARTY_NOTICES.txt
  fetch-resources               Fetch platform media tools for the current OS
  build-windows                 Build the Windows portable application directory
  build-macos [arm64|amd64]     Build the macOS app bundle

Options:
  build-macos --dmg             Also create a DMG`)
}
