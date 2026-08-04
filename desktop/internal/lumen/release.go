package lumen

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"desktop/internal/platform"
)

// OfficialReleaseVersion is deliberately pinned to the Lumen Hub release
// catalog embedded in the Desktop binary. Updating Lumen is an explicit
// Desktop release change, not a mutable "latest" download at runtime.
const OfficialReleaseVersion = "v0.1.1"

const officialReleasePrefix = "https://github.com/EdwinZhanCN/Lumen-Hub/releases/download/"

type ReleaseArtifact struct {
	Version  string
	Profile  string
	FileName string
	URL      string
	SHA256   string
	Binary   string
}

var officialReleaseArtifacts = map[string]ReleaseArtifact{
	"darwin-arm64-cpu": {
		Version: OfficialReleaseVersion, Profile: "darwin-arm64-cpu",
		FileName: "lumen-hub-darwin-arm64-cpu.zip",
		URL:      officialReleasePrefix + OfficialReleaseVersion + "/lumen-hub-darwin-arm64-cpu.zip",
		SHA256:   "38b2b9228747edcc15b56874de63c08d06020b5ccb6f569165e1f85d2c2a4258",
		Binary:   "bin/lumen-hub",
	},
	"darwin-arm64-metal": {
		Version: OfficialReleaseVersion, Profile: "darwin-arm64-metal",
		FileName: "lumen-hub-darwin-arm64-metal.zip",
		URL:      officialReleasePrefix + OfficialReleaseVersion + "/lumen-hub-darwin-arm64-metal.zip",
		SHA256:   "cd379dd8500c1807f6455ce718c4ad0deb440fc03668e4559fd3ed7307b65219",
		Binary:   "bin/lumen-hub",
	},
	"windows-x64-cpu": {
		Version: OfficialReleaseVersion, Profile: "windows-x64-cpu",
		FileName: "lumen-hub-windows-x64-cpu.zip",
		URL:      officialReleasePrefix + OfficialReleaseVersion + "/lumen-hub-windows-x64-cpu.zip",
		SHA256:   "8687708affebe557ea3dd855225b53f618d4eee8ca5cda31e26bfbda9ba83b25",
		Binary:   "bin/lumen-hub.exe",
	},
	"windows-x64-gpu": {
		Version: OfficialReleaseVersion, Profile: "windows-x64-gpu",
		FileName: "lumen-hub-windows-x64-gpu.zip",
		URL:      officialReleasePrefix + OfficialReleaseVersion + "/lumen-hub-windows-x64-gpu.zip",
		SHA256:   "3a1ba438184a7ec8e115044862945c62dbde62299ea874b1fb785772af4654e7",
		Binary:   "bin/lumen-hub.exe",
	},
}

func DefaultReleaseProfile(goos, goarch string) (string, bool) {
	switch {
	case goos == "darwin" && goarch == "arm64":
		return "darwin-arm64-metal", true
	case goos == "windows" && goarch == "amd64":
		return "windows-x64-cpu", true
	default:
		return "", false
	}
}

func CurrentReleaseProfile() (string, bool) {
	return DefaultReleaseProfile(runtime.GOOS, runtime.GOARCH)
}

func ReleaseProfiles(goos, goarch string) []string {
	switch {
	case goos == "darwin" && goarch == "arm64":
		return []string{"darwin-arm64-metal", "darwin-arm64-cpu"}
	case goos == "windows" && goarch == "amd64":
		return []string{"windows-x64-cpu", "windows-x64-gpu"}
	default:
		return nil
	}
}

func CurrentReleaseProfiles() []string {
	return ReleaseProfiles(runtime.GOOS, runtime.GOARCH)
}

type ArtifactDownload func(context.Context, ReleaseArtifact, string) error
type BinaryProbe func(context.Context, string) error

// ReleaseInstaller installs a pinned official Lumen Hub archive. The catalog
// is part of the signed Desktop binary and supplies both the exact release URL
// and digest; no mutable remote manifest participates in the trust decision.
type ReleaseInstaller struct {
	Root      string
	Artifacts map[string]ReleaseArtifact
	Download  ArtifactDownload
	Probe     BinaryProbe
}

func NewOfficialReleaseInstaller(root string) *ReleaseInstaller {
	return &ReleaseInstaller{Root: root, Artifacts: officialReleaseArtifacts}
}

func (i *ReleaseInstaller) Install(ctx context.Context, profile string) (string, error) {
	if i == nil || strings.TrimSpace(i.Root) == "" {
		return "", errors.New("Lumen release installer is not configured")
	}
	artifact, ok := i.Artifacts[profile]
	if !ok {
		return "", fmt.Errorf("unsupported Lumen release profile %q", profile)
	}
	if err := validateReleaseArtifact(artifact); err != nil {
		return "", err
	}
	if current, err := LoadCurrent(i.Root); err == nil && current.Version == artifact.Version && current.Profile == profile {
		return current.Version, nil
	}
	if err := platform.EnsurePrivateDirectory(i.Root); err != nil {
		return "", err
	}
	versions := filepath.Join(i.Root, "versions")
	if err := platform.EnsurePrivateDirectory(versions); err != nil {
		return "", err
	}
	targetName := safeVersionName(artifact.Version) + "-" + safeVersionName(profile)
	target := filepath.Join(versions, targetName)
	if _, err := os.Lstat(target); err == nil {
		return "", errors.New("Lumen target version already exists without a matching current pointer")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	staging, err := os.MkdirTemp(versions, ".staging-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	journal := InstallJournal{
		Phase: "staging", Version: artifact.Version, Profile: profile, TargetHash: artifact.SHA256,
		Target: targetName, Staging: filepath.Base(staging),
	}
	if err := (PackageInstaller{Root: i.Root}).writeJournal(journal); err != nil {
		return "", err
	}
	archivePath := filepath.Join(staging, artifact.FileName)
	download := i.Download
	if download == nil {
		download = downloadOfficialArtifact
	}
	if err := download(ctx, artifact, archivePath); err != nil {
		return "", err
	}
	if err := verifyFileSHA256(archivePath, artifact.SHA256); err != nil {
		return "", err
	}
	payload := filepath.Join(staging, "payload")
	if err := extractReleaseZip(payload, archivePath, "lumen-hub-"+profile); err != nil {
		return "", err
	}
	binaryPath, err := safeJoin(payload, artifact.Binary)
	if err != nil {
		return "", err
	}
	if err := verifyRegular(binaryPath); err != nil {
		return "", fmt.Errorf("Lumen binary is invalid: %w", err)
	}
	if err := os.Chmod(binaryPath, 0o700); err != nil {
		return "", err
	}
	probe := i.Probe
	if probe == nil {
		probe = probeLumenBinary
	}
	if err := probe(ctx, binaryPath); err != nil {
		return "", fmt.Errorf("probe Lumen binary: %w", err)
	}
	journal.Phase = "verified"
	if err := (PackageInstaller{Root: i.Root}).writeJournal(journal); err != nil {
		return "", err
	}
	if err := os.Rename(payload, target); err != nil {
		return "", err
	}
	journal.Phase = "promoting"
	if err := (PackageInstaller{Root: i.Root}).writeJournal(journal); err != nil {
		return "", err
	}
	current := CurrentPointer{
		SchemaVersion: InstallSchemaVersion,
		Version:       artifact.Version,
		Profile:       profile,
		Binary:        filepath.ToSlash(filepath.Join("versions", targetName, artifact.Binary)),
		SHA256:        artifact.SHA256,
	}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return "", err
	}
	if err := platform.WriteAtomic(filepath.Join(i.Root, "current.json"), append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	journal.Phase = "committed"
	if err := (PackageInstaller{Root: i.Root}).writeJournal(journal); err != nil {
		return "", err
	}
	if err := os.Remove(filepath.Join(i.Root, "install.json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return artifact.Version, nil
}

// ReconcileOfficialReleaseInstall completes a promoted official install or
// discards a private staging tree left before promotion. It never guesses a
// version directory: the journal must match the immutable embedded catalog.
func ReconcileOfficialReleaseInstall(root string) error {
	journalPath := filepath.Join(root, "install.json")
	data, err := os.ReadFile(journalPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var journal InstallJournal
	if err := decoder.Decode(&journal); err != nil {
		return fmt.Errorf("decode Lumen install journal: %w", err)
	}
	artifact, ok := officialReleaseArtifacts[journal.Profile]
	if journal.SchemaVersion != InstallSchemaVersion || !ok || journal.Version != artifact.Version || !strings.EqualFold(journal.TargetHash, artifact.SHA256) {
		return errors.New("Lumen install journal does not match the pinned release catalog")
	}
	targetName := safeVersionName(artifact.Version) + "-" + safeVersionName(artifact.Profile)
	if journal.Target != "" && journal.Target != targetName {
		return errors.New("Lumen install journal target does not match the pinned release")
	}
	versions := filepath.Join(root, "versions")
	target, err := safeJoin(versions, targetName)
	if err != nil {
		return err
	}
	if journal.Staging != "" {
		if !strings.HasPrefix(journal.Staging, ".staging-") || filepath.Base(journal.Staging) != journal.Staging {
			return errors.New("invalid Lumen staging directory in install journal")
		}
		staging, err := safeJoin(versions, journal.Staging)
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
	}

	if current, currentErr := LoadCurrent(root); currentErr == nil {
		if current.Version != artifact.Version || current.Profile != artifact.Profile || !strings.EqualFold(current.SHA256, artifact.SHA256) {
			return errors.New("Lumen install journal conflicts with current pointer")
		}
		return os.Remove(journalPath)
	}
	binaryPath, binaryErr := safeJoin(target, artifact.Binary)
	if binaryErr == nil {
		binaryErr = verifyRegular(binaryPath)
	}
	if binaryErr == nil {
		current := CurrentPointer{
			SchemaVersion: InstallSchemaVersion, Version: artifact.Version, Profile: artifact.Profile,
			Binary: filepath.ToSlash(filepath.Join("versions", targetName, artifact.Binary)), SHA256: artifact.SHA256,
		}
		pointer, err := json.MarshalIndent(current, "", "  ")
		if err != nil {
			return err
		}
		if err := platform.WriteAtomic(filepath.Join(root, "current.json"), append(pointer, '\n'), 0o600); err != nil {
			return err
		}
		return os.Remove(journalPath)
	}
	if journal.Phase == "staging" || journal.Phase == "verified" {
		return os.Remove(journalPath)
	}
	return fmt.Errorf("Lumen %s install cannot be reconciled: %w", journal.Phase, binaryErr)
}

func validateReleaseArtifact(artifact ReleaseArtifact) error {
	if artifact.Version != OfficialReleaseVersion || artifact.Profile == "" || artifact.FileName == "" || artifact.Binary == "" {
		return errors.New("incomplete Lumen release artifact")
	}
	if artifact.URL != officialReleasePrefix+artifact.Version+"/"+artifact.FileName {
		return errors.New("Lumen release artifact URL is not an exact official release URL")
	}
	if len(artifact.SHA256) != sha256.Size*2 {
		return errors.New("invalid Lumen release artifact SHA-256")
	}
	if _, err := hex.DecodeString(artifact.SHA256); err != nil {
		return fmt.Errorf("invalid Lumen release artifact SHA-256: %w", err)
	}
	return validateRelativePath(artifact.Binary)
}

func downloadOfficialArtifact(ctx context.Context, artifact ReleaseArtifact, target string) error {
	if err := validateReleaseArtifact(artifact); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Lumilio-Photos-Desktop/"+OfficialReleaseVersion)
	client := &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: 30 * time.Second,
	}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download Lumen artifact: HTTP %s", response.Status)
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, response.Body)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func verifyFileSHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("Lumen artifact digest %s does not match pinned digest %s", actual, expected)
	}
	return nil
}

func extractReleaseZip(root, archivePath, expectedArchiveRoot string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	if err := platform.EnsurePrivateDirectory(root); err != nil {
		return err
	}
	for _, item := range archive.File {
		name := strings.TrimSuffix(item.Name, "/")
		if strings.Contains(name, "\\") {
			return fmt.Errorf("Lumen archive contains a non-portable path %q", item.Name)
		}
		parts := strings.Split(name, "/")
		if len(parts) == 0 || parts[0] != expectedArchiveRoot {
			return fmt.Errorf("Lumen archive entry %q is outside expected root %q", item.Name, expectedArchiveRoot)
		}
		if len(parts) == 1 {
			continue
		}
		relative := strings.Join(parts[1:], "/")
		if err := validateRelativePath(relative); err != nil {
			return err
		}
		target, err := safeJoin(root, relative)
		if err != nil {
			return err
		}
		if item.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if item.Mode()&os.ModeSymlink != 0 || !item.Mode().IsRegular() {
			return fmt.Errorf("Lumen archive contains a non-regular entry %q", item.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		input, err := item.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeOutputErr := output.Close()
		closeInputErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
	}
	return nil
}

func probeLumenBinary(ctx context.Context, binary string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open(binary)
	if err != nil {
		return err
	}
	defer file.Close()
	var header [4]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return err
	}
	// Lumen Desktop releases are PE32+ on Windows and Mach-O 64-bit on
	// macOS. ELF is accepted for the same installer implementation's future
	// Linux use. Runtime identity is proved separately through control gRPC;
	// executing --help is unsafe because GPU builds initialise their adapter
	// before parsing CLI arguments.
	isPE := header[0] == 'M' && header[1] == 'Z'
	isMachO64 := (header == [4]byte{0xcf, 0xfa, 0xed, 0xfe}) || (header == [4]byte{0xfe, 0xed, 0xfa, 0xcf})
	isELF := header == [4]byte{0x7f, 'E', 'L', 'F'}
	if !isPE && !isMachO64 && !isELF {
		return fmt.Errorf("Lumen binary has an unsupported executable header %x", header)
	}
	return nil
}
