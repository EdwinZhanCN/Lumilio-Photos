// Command desktop is the Wails v3 entry point for the Lumilio Photos desktop
// app. It manages a private, bundled PostgreSQL instance and runs the existing
// Go API server in-process (see the supervisor package), while the React UI is
// served over HTTP and opened in the user's browser.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"server/app"
)

func main() {
	controls, err := parseDesktopCLI(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := newDesktopApp(controls).run(); err != nil {
		log.Printf("application exited with error: %v", err)
		os.Exit(1)
	}
}

func parseDesktopCLI(args []string, stderr io.Writer) (app.OperatorControls, error) {
	flags := flag.NewFlagSet("Lumilio Photos", flag.ContinueOnError)
	flags.SetOutput(stderr)
	breakGlass := flags.Bool("break-glass", false, "recover an active administrator for this launch")
	username := flags.String("break-glass-username", "", "active administrator username to recover")
	if err := flags.Parse(args); err != nil {
		return app.OperatorControls{}, err
	}
	if flags.NArg() != 0 {
		return app.OperatorControls{}, fmt.Errorf("unexpected positional arguments")
	}
	trimmedUsername := strings.TrimSpace(*username)
	if trimmedUsername != "" && !*breakGlass {
		return app.OperatorControls{}, fmt.Errorf("--break-glass-username requires --break-glass")
	}
	return app.OperatorControls{BreakGlass: *breakGlass, BreakGlassUsername: trimmedUsername}, nil
}
