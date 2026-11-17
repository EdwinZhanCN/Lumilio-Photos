// Command terminology checks the product vocabulary everywhere a person
// reads it: the canonical Repository, Storage Location, and Lumen capability
// terms in the Web, Desktop, docs, READMEs, and API annotations, and literal
// en/zh copy for every closed code set the API contract exposes. It is owned
// by the lumilio-frontend-i18n skill; run it with task terminology:check.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	root, err := gitRoot()
	if err != nil {
		fail(err)
	}
	if err := checkUserFacingTerminology(root); err != nil {
		fail(err)
	}
	if err := checkContractEnumCoverage(root); err != nil {
		fail(err)
	}
	fmt.Println("terminology: canonical terms and contract copy are complete")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func gitRoot() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
