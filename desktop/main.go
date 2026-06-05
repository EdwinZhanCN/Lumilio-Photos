// Command desktop is the Wails v3 entry point for the Lumilio Photos desktop
// app. It manages a private, bundled PostgreSQL instance and runs the existing
// Go API server in-process (see the supervisor package), while the React UI is
// served over HTTP and opened in the user's browser.
package main

import (
	"log"
	"os"
)

func main() {
	if err := newDesktopApp().run(); err != nil {
		log.Printf("application exited with error: %v", err)
		os.Exit(1)
	}
}
