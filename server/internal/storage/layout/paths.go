package layout

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Every private path is built here, one constructor per kind. An argument
// that is not a single valid path component is an error, so no caller can
// reach outside the kind it asked for.

const (
	studioDir        = "user/studio"
	studioSuffix     = ".lumilio-sidecar"
	assetSidecarDir  = "user/assets"
	assetSidecarExt  = ".json"
	trashFilesDir    = "trash/files"
	trashInfoDir     = "trash/info"
	artifactsDir     = "cache/artifacts"
	cacheDirTag      = "cache/CACHEDIR.TAG"
	uploadsDir       = "work/uploads"
	tempDir          = "work/tmp"
	quarantineUpload = "quarantine/uploads"
	recoveryDir      = "quarantine/recovery"
	uploadManifest   = "session.json"
	uploadMerged     = "merged"
	uploadChunk      = "chunk-"
)

func join(elements ...string) (Path, error) {
	value := path.Join(append([]string{PrivateRoot}, elements...)...)
	if _, err := parse(value); err != nil {
		return Path{}, err
	}
	return private(value)
}

func joinSegments(dir string, segments ...string) (Path, error) {
	for _, value := range segments {
		if !segment(value) {
			return Path{}, fmt.Errorf("%w: %q is not a single path component", ErrPathInvalid, value)
		}
	}
	return join(append([]string{dir}, segments...)...)
}

// Root is .lumilio/ itself.
func Root() Path { return Path{value: PrivateRoot, space: namespacePrivate} }

// ClassDir is the directory of one class.
func ClassDir(class Class) (Path, error) {
	if PolicyOf(class).Class == Unknown {
		return Path{}, fmt.Errorf("%w: %q is not a class", ErrPathNamespace, class)
	}
	return join(string(class))
}

// Studio is the Studio record of one Asset: user/studio/<assetID>.lumilio-sidecar.
func Studio(assetID uuid.UUID) (Path, error) {
	return joinSegments(studioDir, assetID.String()+studioSuffix)
}

// StudioDir holds every Studio record.
func StudioDir() Path { return mustJoin(studioDir) }

// StudioAssetID returns the Asset ID a Studio record path names.
func StudioAssetID(p Path) (uuid.UUID, bool) {
	if !p.IsPrivate() || path.Dir(p.value) != path.Join(PrivateRoot, studioDir) {
		return uuid.Nil, false
	}
	name, ok := strings.CutSuffix(path.Base(p.value), studioSuffix)
	if !ok || name == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(name)
	return id, err == nil && id.String() == name
}

// AssetSidecar is the portable sidecar of one media entry:
// user/assets/<entry path>.json.
func AssetSidecar(entry Path) (Path, error) {
	if !entry.IsMedia() {
		return Path{}, fmt.Errorf("%w: an asset sidecar names a media path, not %q", ErrPathNamespace, entry.value)
	}
	return join(assetSidecarDir, entry.value+assetSidecarExt)
}

// AssetSidecarDir holds every asset sidecar.
func AssetSidecarDir() Path { return mustJoin(assetSidecarDir) }

// TrashDir holds one trashed original: trash/files/<trashID>/.
func TrashDir(trashID uuid.UUID) (Path, error) {
	if trashID == uuid.Nil {
		return Path{}, fmt.Errorf("%w: zero trash ID", ErrPathInvalid)
	}
	return joinSegments(trashFilesDir, trashID.String())
}

// TrashFile is a trashed original under its own name:
// trash/files/<trashID>/<name>.
func TrashFile(trashID uuid.UUID, name string) (Path, error) {
	if trashID == uuid.Nil {
		return Path{}, fmt.Errorf("%w: zero trash ID", ErrPathInvalid)
	}
	return joinSegments(trashFilesDir, trashID.String(), name)
}

// TrashInfo describes one trashed original: trash/info/<trashID>.json.
func TrashInfo(trashID uuid.UUID) (Path, error) {
	if trashID == uuid.Nil {
		return Path{}, fmt.Errorf("%w: zero trash ID", ErrPathInvalid)
	}
	return joinSegments(trashInfoDir, trashID.String()+".json")
}

// TrashFilesDir and TrashInfoDir are the two halves of the trash.
func TrashFilesDir() Path { return mustJoin(trashFilesDir) }
func TrashInfoDir() Path  { return mustJoin(trashInfoDir) }

// ArtifactID identifies one immutable derivative: the content fence of its
// source, the pipeline stage, the pipeline version, and the file name. The
// catalog stores this identity, never the path.
type ArtifactID struct{ SourceFence, Stage, PipelineVersion, Name string }

// Artifact is cache/artifacts/<source fence>/<stage>/<pipeline version>/<name>.
func Artifact(id ArtifactID) (Path, error) {
	return joinSegments(artifactsDir, id.SourceFence, id.Stage, id.PipelineVersion, id.Name)
}

// ArtifactsDir holds every artifact.
func ArtifactsDir() Path { return mustJoin(artifactsDir) }

// UploadSession is the one directory of a chunked upload session:
// work/uploads/<sessionID>/. Ending the session removes it.
func UploadSession(sessionID uuid.UUID) (Path, error) {
	return joinSegments(uploadsDir, sessionID.String())
}

// UploadManifest is work/uploads/<sessionID>/session.json.
func UploadManifest(sessionID uuid.UUID) (Path, error) {
	return joinSegments(uploadsDir, sessionID.String(), uploadManifest)
}

// UploadChunk is work/uploads/<sessionID>/chunk-<index>.
func UploadChunk(sessionID uuid.UUID, index int) (Path, error) {
	if index < 0 {
		return Path{}, fmt.Errorf("%w: chunk index %d", ErrPathInvalid, index)
	}
	return joinSegments(uploadsDir, sessionID.String(), uploadChunk+strconv.Itoa(index))
}

// UploadMerged is work/uploads/<sessionID>/merged, the assembled upload.
func UploadMerged(sessionID uuid.UUID) (Path, error) {
	return joinSegments(uploadsDir, sessionID.String(), uploadMerged)
}

// UploadsDir holds every upload session.
func UploadsDir() Path { return mustJoin(uploadsDir) }

// QuarantineUpload keeps a failed upload for a human decision:
// quarantine/uploads/<id>/.
func QuarantineUpload(id uuid.UUID) (Path, error) {
	return joinSegments(quarantineUpload, id.String())
}

// RecoveryReceipt is quarantine/recovery/<receiptID>.json.
func RecoveryReceipt(receiptID uuid.UUID) (Path, error) {
	return joinSegments(recoveryDir, receiptID.String()+".json")
}

// RecoveryReceiptDir holds the bytes one registration moved:
// quarantine/recovery/<receiptID>/.
func RecoveryReceiptDir(receiptID uuid.UUID) (Path, error) {
	return joinSegments(recoveryDir, receiptID.String())
}

// RecoveryDir holds every receipt.
func RecoveryDir() Path { return mustJoin(recoveryDir) }

// Log is one file under logs/, such as operations.log or a rotated
// operations.log.1.
func Log(name string) (Path, error) { return joinSegments(string(Logs), name) }

// Temp is a fresh name for an atomic-write temporary: work/tmp/<uuid>.
// Temporaries are created beside nothing they replace, so an interrupted
// write leaves its bytes in work/, which startup clears.
func Temp() Path { return mustJoin(tempDir, uuid.NewString()) }

// TempDir holds atomic-write temporaries.
func TempDir() Path { return mustJoin(tempDir) }

// RootTemp is a fresh temporary name for a Repository or Storage Location
// root: a probe or a marker temporary.
func RootTemp(purpose string) (string, error) {
	if !segment(purpose) {
		return "", fmt.Errorf("%w: temporary purpose %q", ErrPathInvalid, purpose)
	}
	return TempPrefix + purpose + "-" + uuid.NewString(), nil
}

func mustJoin(elements ...string) Path {
	p, err := join(elements...)
	if err != nil {
		panic(err)
	}
	return p
}
