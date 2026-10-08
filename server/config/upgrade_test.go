package config

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// renameListenStep is a test-only step: a hypothetical schema v1 called the
// application listener server.legacy_listen, and v2 calls it server.listen.
var renameListenStep = manifestStep{
	From: 1,
	Apply: func(tree map[string]any) error {
		server := tree["server"].(map[string]any)
		server["listen"] = server["legacy_listen"]
		delete(server, "legacy_listen")
		return nil
	},
}

var legacyManifest = strings.Replace(completeManifest, "\nlisten = ", "\nlegacy_listen = ", 1)

func TestUpgradeManifestAppliesStepsAndRendersAStrictManifest(t *testing.T) {
	upgrade, err := upgradeManifest([]byte(legacyManifest), []manifestStep{renameListenStep}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if upgrade.From != 1 || upgrade.To != 2 || upgrade.Data == nil {
		t.Fatalf("upgrade = %+v", upgrade)
	}
	for _, want := range []string{"#:schema " + SchemaID, "Upgraded from schema v1", "schema_version = 2", `listen = "127.0.0.1:6680"`} {
		if !bytes.Contains(upgrade.Data, []byte(want)) {
			t.Fatalf("upgraded manifest lacks %q:\n%s", want, upgrade.Data)
		}
	}
	if bytes.Contains(upgrade.Data, []byte("legacy_listen")) {
		t.Fatalf("upgraded manifest still carries the old key:\n%s", upgrade.Data)
	}
	var raw manifest
	if err := toml.NewDecoder(bytes.NewReader(upgrade.Data)).DisallowUnknownFields().Decode(&raw); err != nil {
		t.Fatalf("upgraded manifest fails the strict decode: %v", err)
	}
	if problems := validateManifestPresence(raw); len(problems) != 0 {
		t.Fatalf("upgraded manifest is incomplete: %v", problems)
	}
}

func TestUpgradeManifestRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		input   string
		steps   []manifestStep
		current int
		want    string
	}{
		"pre-release or newer": {input: strings.Replace(completeManifest, "schema_version = 1", "schema_version = 6", 1), current: 2, want: "pre-release build"},
		"invalid version":      {input: strings.Replace(completeManifest, "schema_version = 1", "schema_version = 0", 1), current: 2, want: "versions start at 1"},
		"missing step":         {input: legacyManifest, current: 2, want: "no upgrade step from schema_version 1"},
		"step leaves old key": {
			input:   legacyManifest,
			steps:   []manifestStep{{From: 1, Apply: func(map[string]any) error { return nil }}},
			current: 2,
			want:    "does not match schema v2",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := upgradeManifest([]byte(tc.input), tc.steps, tc.current)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestUpgradeManifestLeavesCurrentManifestAlone(t *testing.T) {
	upgrade, err := UpgradeManifest([]byte(completeManifest))
	if err != nil {
		t.Fatal(err)
	}
	if upgrade.Data != nil || upgrade.From != SchemaVersion || upgrade.To != SchemaVersion {
		t.Fatalf("upgrade of a current manifest = %+v, want no data", upgrade)
	}
}

func TestOlderManifestPointsAtConfigUpgrade(t *testing.T) {
	err := checkManifestVersion([]byte(completeManifest), 2)
	if err == nil || !strings.Contains(err.Error(), "older than this build (2)") || !strings.Contains(err.Error(), "server config upgrade") {
		t.Fatalf("older manifest error = %v", err)
	}
}
