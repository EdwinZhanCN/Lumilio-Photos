package config

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// manifestStep upgrades a parsed manifest tree from schema_version From to
// From+1. The runner stamps schema_version; a step only moves, renames, or
// adds keys, writing every new key explicitly (there are no code defaults).
type manifestStep struct {
	From  int
	Apply func(tree map[string]any) error
}

// manifestSteps is the production upgrade path from the rc.1 baseline. It is
// empty until the first schema_version after 1; a step added here must also
// bump SchemaVersion.
var manifestSteps []manifestStep

// ManifestUpgrade is the result of UpgradeManifest. Data is nil when the
// manifest is already at the current schema_version.
type ManifestUpgrade struct {
	From, To int
	Data     []byte
}

// UpgradeManifest rewrites a manifest at an older supported schema_version to
// the current one. It never touches the file: `server config upgrade` owns the
// write and the .bak copy. A pre-release or newer manifest is refused.
func UpgradeManifest(data []byte) (ManifestUpgrade, error) {
	return upgradeManifest(data, manifestSteps, SchemaVersion)
}

func upgradeManifest(data []byte, steps []manifestStep, current int) (ManifestUpgrade, error) {
	var tree map[string]any
	if err := toml.Unmarshal(data, &tree); err != nil {
		return ManifestUpgrade{}, fmt.Errorf("parse runtime manifest: %w", err)
	}
	rawVersion, ok := tree["schema_version"].(int64)
	if !ok {
		return ManifestUpgrade{}, errors.New("runtime manifest has no integer schema_version")
	}
	from := int(rawVersion)
	if err := checkVersion(from, current); err != nil {
		return ManifestUpgrade{}, err
	}
	if from == current {
		return ManifestUpgrade{From: from, To: current}, nil
	}

	for version := from; version < current; version++ {
		step, found := findManifestStep(steps, version)
		if !found {
			return ManifestUpgrade{}, fmt.Errorf("no upgrade step from schema_version %d", version)
		}
		if err := step.Apply(tree); err != nil {
			return ManifestUpgrade{}, fmt.Errorf("upgrade schema_version %d to %d: %w", version, version+1, err)
		}
		tree["schema_version"] = int64(version + 1)
	}

	encoded, err := toml.Marshal(tree)
	if err != nil {
		return ManifestUpgrade{}, fmt.Errorf("encode upgraded manifest: %w", err)
	}
	var raw manifest
	if err := toml.NewDecoder(bytes.NewReader(encoded)).DisallowUnknownFields().Decode(&raw); err != nil {
		return ManifestUpgrade{}, fmt.Errorf("upgraded manifest does not match schema v%d: %w", current, err)
	}
	if problems := validateManifestPresence(raw); len(problems) != 0 {
		return ManifestUpgrade{}, fmt.Errorf("upgraded manifest is incomplete: %w", invalidConfig(problems))
	}
	rendered, err := encodeUpgradedManifest(raw, from, current)
	if err != nil {
		return ManifestUpgrade{}, err
	}
	return ManifestUpgrade{From: from, To: current, Data: rendered}, nil
}

func findManifestStep(steps []manifestStep, from int) (manifestStep, bool) {
	for _, step := range steps {
		if step.From == from {
			return step, true
		}
	}
	return manifestStep{}, false
}

// checkManifestVersion reads only schema_version, before the strict decode,
// so a manifest from an older, newer, or pre-release build is reported as such
// instead of as a list of unknown or missing fields. A missing or malformed
// version is left to the strict decode and presence checks.
func checkManifestVersion(data []byte, current int) error {
	var probe struct {
		SchemaVersion *int `toml:"schema_version"`
	}
	if err := toml.Unmarshal(data, &probe); err != nil || probe.SchemaVersion == nil {
		return nil
	}
	if *probe.SchemaVersion < current && *probe.SchemaVersion >= 1 {
		return fmt.Errorf(
			"schema_version = %d is older than this build (%d): run `server config upgrade --config <this file>` to upgrade it; the current file is kept as a .bak copy",
			*probe.SchemaVersion,
			current,
		)
	}
	return checkVersion(*probe.SchemaVersion, current)
}

// checkVersion rejects versions this build cannot read or upgrade. Pre-release
// manifests used 1–6 before the rc.1 reset, so a number above current may be
// either a newer build or a pre-release one.
func checkVersion(version, current int) error {
	if version > current {
		return fmt.Errorf(
			"schema_version = %d is newer than this build supports (%d): the file was written for a newer Lumilio Photos, or by a pre-release build whose configuration is not migrated (create a new one from the current examples)",
			version,
			current,
		)
	}
	if version < 1 {
		return fmt.Errorf("schema_version = %d is invalid; versions start at 1", version)
	}
	return nil
}
