package imaging

import (
	"os"
	"os/exec"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

func TestStartVipsPropagatesInitializationFailure(t *testing.T) {
	// Shutdown is irreversible. Keep the failure probe out of the parent's
	// process-global runtime so the image regression tests remain independent.
	if os.Getenv("LUMILIO_TEST_VIPS_STOPPED") == "1" {
		if err := vips.Startup(nil); err != nil {
			t.Fatal(err)
		}
		vips.Shutdown()
		if err := StartVips(); err == nil {
			t.Fatal("native initialization failure was ignored")
		}
		if err := StartVips(); err == nil {
			t.Fatal("repeated startup lost the initialization failure")
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStartVipsPropagatesInitializationFailure$")
	cmd.Env = append(os.Environ(), "LUMILIO_TEST_VIPS_STOPPED=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("startup failure probe: %v\n%s", err, out)
	}
}
