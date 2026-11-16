package layout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseMedia(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"Trips/2026/photo.jpg", "inbox/2026/08/upload.heic", ".hidden/photo.jpg", "photo.jpg", ".lumilio-dev/photo.jpg"} {
		got, err := ParseMedia(value)
		if err != nil || got.String() != value || !got.IsMedia() {
			t.Errorf("ParseMedia(%q) = %q media=%t, %v", value, got, got.IsMedia(), err)
		}
	}
}

func TestParseMediaRejectsNonCanonicalAndReservedPaths(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", ".", "../photo.jpg", "Trips/../photo.jpg", "/photo.jpg", `C:\\photo.jpg`, `Trips\\photo.jpg`,
		"Trips//photo.jpg", "Trips/photo.jpg/", "Trips/CON.jpg", "Trips/photo.jpg ", "Trips/photo?.jpg",
		"Trips/\x00photo.jpg", ".lumilio", ".lumilio/cache/photo.jpg", ".lumiliorepo", ".lumiliorepo.lock",
		".lumilioroot", ".lumilioroot.lock", ".lumilio-tmp-probe-1",
	} {
		if _, err := ParseMedia(value); err == nil {
			t.Errorf("ParseMedia(%q) unexpectedly succeeded", value)
		}
	}
}

func TestParseKeepsPrivatePathsPrivate(t *testing.T) {
	t.Parallel()
	got, err := Parse(".lumilio/work/uploads/x/session.json")
	if err != nil || !got.IsPrivate() {
		t.Fatalf("Parse = %q private=%t, %v", got, got.IsPrivate(), err)
	}
	for _, value := range []string{"inbox/upload.jpg", ".lumiliofoo/x", "../.lumilio/x"} {
		if _, err := Parse(value); err == nil {
			t.Errorf("Parse(%q) unexpectedly succeeded", value)
		}
	}
	if _, err := Parse("inbox/upload.jpg"); !errors.Is(err, ErrPathNamespace) {
		t.Fatalf("error = %v, want a namespace error", err)
	}
}

func TestConstructorsBuildTheFrozenLayout(t *testing.T) {
	t.Parallel()
	asset := uuid.MustParse("6f9619ff-8b86-d011-b42d-00c04fc964ff")
	session := uuid.MustParse("1b4e28ba-2fa1-11d2-883f-0016d3cca427")
	entry, err := ParseMedia("Trips/2026/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	must := func(p Path, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if !p.IsPrivate() {
			t.Fatalf("%q is not private", p)
		}
		return p.String()
	}
	for got, want := range map[string]string{
		must(Studio(asset)):                   ".lumilio/user/studio/6f9619ff-8b86-d011-b42d-00c04fc964ff.lumilio-sidecar",
		must(AssetSidecar(entry)):             ".lumilio/user/assets/Trips/2026/photo.jpg.json",
		must(TrashDir(session)):               ".lumilio/trash/files/1b4e28ba-2fa1-11d2-883f-0016d3cca427",
		must(TrashFile(session, "IMG 1.jpg")): ".lumilio/trash/files/1b4e28ba-2fa1-11d2-883f-0016d3cca427/IMG 1.jpg",
		must(TrashInfo(session)):              ".lumilio/trash/info/1b4e28ba-2fa1-11d2-883f-0016d3cca427.json",
		must(Artifact(ArtifactID{"fence", "faces", "v3", "crop-1.webp"})): ".lumilio/cache/artifacts/fence/faces/v3/crop-1.webp",
		must(UploadSession(session)):                                      ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427",
		must(UploadManifest(session)):                                     ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/session.json",
		must(UploadChunk(session, 3)):                                     ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/chunk-3",
		must(UploadFile(session, "IMG 1.jpg")):                            ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/IMG 1.jpg",
		must(UploadFile(session, "session.json")):                         ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/upload-session.json",
		must(UploadFile(session, "chunk-1")):                              ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/upload-chunk-1",
		must(UploadFile(session, "chunk-1.jpg")):                          ".lumilio/work/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/chunk-1.jpg",
		must(QuarantineUploadFile(session, "IMG 1.jpg")):                  ".lumilio/quarantine/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427/IMG 1.jpg",
		must(QuarantineUpload(session)):                                   ".lumilio/quarantine/uploads/1b4e28ba-2fa1-11d2-883f-0016d3cca427",
		must(RecoveryReceipt(session)):                                    ".lumilio/quarantine/recovery/1b4e28ba-2fa1-11d2-883f-0016d3cca427.json",
		must(RecoveryReceiptDir(session)):                                 ".lumilio/quarantine/recovery/1b4e28ba-2fa1-11d2-883f-0016d3cca427",
		must(Log("operations.log.1")):                                     ".lumilio/logs/operations.log.1",
		must(ClassDir(Cache)):                                             ".lumilio/cache",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if temp := Temp().String(); !strings.HasPrefix(temp, ".lumilio/work/tmp/") || Temp() == Temp() {
		t.Errorf("Temp() = %q, want a fresh name under work/tmp", temp)
	}
	if id, ok := StudioAssetID(Path{value: must(Studio(asset)), space: namespacePrivate}); !ok || id != asset {
		t.Errorf("StudioAssetID = %v %t", id, ok)
	}
}

func TestConstructorsRejectArgumentsThatLeaveTheirKind(t *testing.T) {
	t.Parallel()
	for name, build := range map[string]func() (Path, error){
		"trash traversal":    func() (Path, error) { return TrashFile(uuid.New(), "..") },
		"trash nested":       func() (Path, error) { return TrashFile(uuid.New(), "a/b") },
		"trash empty name":   func() (Path, error) { return TrashFile(uuid.New(), "") },
		"trash zero ID":      func() (Path, error) { return TrashInfo(uuid.Nil) },
		"trash dir zero ID":  func() (Path, error) { return TrashDir(uuid.Nil) },
		"artifact traversal": func() (Path, error) { return Artifact(ArtifactID{"f", "..", "v", "n"}) },
		"artifact empty":     func() (Path, error) { return Artifact(ArtifactID{"f", "s", "", "n"}) },
		"artifact reserved":  func() (Path, error) { return Artifact(ArtifactID{"f", "s", "v", "CON"}) },
		"log nested":         func() (Path, error) { return Log("../trash/x") },
		"negative chunk":     func() (Path, error) { return UploadChunk(uuid.New(), -1) },
		"private sidecar":    func() (Path, error) { return AssetSidecar(LogsDir()) },
		"child of media":     func() (Path, error) { m, _ := ParseMedia("a"); return m.Child("b") },
		"child traversal":    func() (Path, error) { return LogsDir().Child("..") },
		"unknown class":      func() (Path, error) { return ClassDir("assets") },
	} {
		if p, err := build(); err == nil {
			t.Errorf("%s: built %q", name, p)
		}
	}
}

func TestParseStagedAcceptsOnlyStagedFiles(t *testing.T) {
	t.Parallel()
	session := "1b4e28ba-2fa1-11d2-883f-0016d3cca427"
	for _, value := range []string{".lumilio/work/uploads/" + session + "/IMG.jpg", ".lumilio/quarantine/uploads/" + session + "/IMG.jpg"} {
		p, err := ParseStaged(value)
		if id, ok := StagedSession(p); err != nil || !ok || id.String() != session {
			t.Errorf("ParseStaged(%q) = %v %t, %v", value, id, ok, err)
		}
	}
	for _, value := range []string{".lumilio/work/uploads/not-a-uuid/x", ".lumilio/work/uploads/" + session, ".lumilio/work/uploads/" + session + "/a/b", ".lumilio/work/tmp/x"} {
		if _, err := ParseStaged(value); err == nil {
			t.Errorf("ParseStaged(%q) succeeded", value)
		}
	}
	original, _ := Parse(".lumilio/work/uploads/" + session + "/IMG.jpg")
	moved, err := RecoveryMoved(uuid.MustParse(session), original)
	if err != nil || moved.String() != ".lumilio/quarantine/recovery/"+session+"/work/uploads/"+session+"/IMG.jpg" {
		t.Fatalf("RecoveryMoved = %q, %v", moved, err)
	}
	if child, err := LogsDir().Child("error.log"); err != nil || child.String() != ".lumilio/logs/error.log" {
		t.Fatalf("Child = %q, %v", child, err)
	}
	if TempPattern("volume-probe") != ".lumilio-tmp-volume-probe-*" {
		t.Fatal(TempPattern("volume-probe"))
	}
}

func TestClassifyUsesTheFirstSegment(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]Class{
		".lumilio/user/studio/x.lumilio-sidecar": User,
		".lumilio/trash/files/t":                 Trash,
		".lumilio/cache/artifacts/f/s/v/n":       Cache,
		".lumilio/cache/CACHEDIR.TAG":            Cache,
		".lumilio/work/tmp/x":                    Work,
		".lumilio/quarantine/recovery/r.json":    Quarantine,
		".lumilio/logs/error.log":                Logs,
		".lumilio/staging/incoming/x":            Unknown,
		".lumilio/assets/faces/x.jpg":            Unknown,
		".lumilio/cachex/y":                      Unknown,
		".lumilio":                               Unknown,
	} {
		p, err := Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if got := Classify(p); got != want {
			t.Errorf("Classify(%q) = %q, want %q", value, got, want)
		}
	}
	media, _ := ParseMedia("user/photo.jpg")
	if got := Classify(media); got != Unknown {
		t.Errorf("a media path classified as %q", got)
	}
}

func TestPoliciesMatchTheFrozenTable(t *testing.T) {
	t.Parallel()
	var classes []Class
	for _, policy := range Policies {
		classes = append(classes, policy.Class)
		wantPrivate := policy.Class == Work || policy.Class == Quarantine
		if got := policy.Perm == 0o700; got != wantPrivate {
			t.Errorf("%s perm %o", policy.Class, policy.Perm)
		}
	}
	if !slices.Equal(classes, []Class{User, Trash, Cache, Work, Quarantine, Logs}) {
		t.Fatalf("classes = %v", classes)
	}
	if PolicyOf(Cache).Backup != BackupExclude || PolicyOf(Work).Backup != BackupExclude || PolicyOf(User).Backup != BackupInclude {
		t.Error("backup policy differs from the layout")
	}
	if PolicyOf(Quarantine).Created != CreatedLazily || PolicyOf(Unknown).Cleanup != NeverAutomatic {
		t.Error("creation or unknown policy differs from the layout")
	}
}

func TestIsReservedAndRootTemp(t *testing.T) {
	t.Parallel()
	for _, value := range []string{".lumilio", ".lumilio/x", ".lumiliorepo", ".lumiliorepo.lock", ".lumilioroot", ".lumilioroot.lock", ".lumilio-tmp-probe-x"} {
		if !IsReserved(value) {
			t.Errorf("IsReserved(%q) = false", value)
		}
	}
	for _, value := range []string{"inbox", "photos/.lumilio", ".lumilio-dev", "lumilio"} {
		if IsReserved(value) {
			t.Errorf("IsReserved(%q) = true", value)
		}
	}
	name, err := RootTemp("volume-probe")
	if err != nil || !IsTemp(name) || !IsReserved(name) {
		t.Fatalf("RootTemp = %q, %v", name, err)
	}
	if _, err := RootTemp("a/b"); err == nil {
		t.Fatal("RootTemp accepted a nested purpose")
	}
}

func TestIsRooted(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"/absolute/photo.jpg", "/", `\Windows\System32`, `\`, `C:\Users\someone`, "C:/Users/someone", `C:relative`, `\\server\share\photo.jpg`} {
		if !IsRooted(value) {
			t.Errorf("IsRooted(%q) = false", value)
		}
	}
	for _, value := range []string{"", "inbox/2026/07/photo.jpg", "photo.jpg", "./photo.jpg", "../escape.jpg", "album/.hidden/photo.jpg"} {
		if IsRooted(value) {
			t.Errorf("IsRooted(%q) = true", value)
		}
	}
}

func TestInitCreatesExactlyTheLayoutAndValidateAcceptsIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || name == root {
			return err
		}
		relative, _ := filepath.Rel(root, name)
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		got = append(got, filepath.ToSlash(relative)+suffix)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		".lumilio/", ".lumilio/cache/", ".lumilio/cache/CACHEDIR.TAG", ".lumilio/cache/artifacts/",
		".lumilio/logs/", ".lumilio/logs/error.log", ".lumilio/logs/operations.log",
		".lumilio/trash/", ".lumilio/trash/files/", ".lumilio/trash/info/",
		".lumilio/user/", ".lumilio/user/assets/", ".lumilio/user/studio/",
		".lumilio/work/", ".lumilio/work/tmp/", ".lumilio/work/uploads/", "inbox/",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tree:\n got %q\nwant %q", got, want)
	}
	tag, err := os.ReadFile(filepath.Join(root, ".lumilio", "cache", "CACHEDIR.TAG"))
	if err != nil || !strings.HasPrefix(string(tag), "Signature: 8a477f597d28d172789f06886806bc55") {
		t.Fatalf("CACHEDIR.TAG = %q, %v", tag, err)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{"work", "work/uploads", "work/tmp"} {
			info, err := os.Stat(filepath.Join(root, ".lumilio", filepath.FromSlash(name)))
			if err != nil || info.Mode().Perm() != 0o700 {
				t.Errorf("%s mode = %v, %v", name, info.Mode().Perm(), err)
			}
		}
	}
	report, err := Validate(root)
	if err != nil || !report.Repairable() || len(report.Missing)+len(report.Unknown) != 0 {
		t.Fatalf("Validate = %+v, %v", report, err)
	}
}

func TestInitRepairsAndKeepsExistingData(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, ".lumilio", "logs", "operations.log")
	if err := os.WriteFile(logPath, []byte("{\"kept\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".lumilio", "trash")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".lumilio", "staging", "incoming"), 0o700); err != nil {
		t.Fatal(err)
	}
	report, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Missing, []string{".lumilio/trash", ".lumilio/trash/files", ".lumilio/trash/info"}) || !slices.Equal(report.Unknown, []string{".lumilio/staging"}) || !report.Repairable() {
		t.Fatalf("report = %+v", report)
	}
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(logPath); string(data) != "{\"kept\":true}\n" {
		t.Fatalf("Init truncated a log: %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, ".lumilio", "staging", "incoming")); err != nil {
		t.Fatalf("Init touched an unknown directory: %v", err)
	}
	report, _ = Validate(root)
	if len(report.Missing) != 0 || !slices.Equal(report.Unknown, []string{".lumilio/staging"}) {
		t.Fatalf("after repair: %+v", report)
	}
}

func TestValidateReportsAFileWhereADirectoryBelongs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, ".lumilio", "cache", "artifacts")
	if err := os.RemoveAll(cache); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Validate(root)
	if err != nil || report.Repairable() || !slices.Equal(report.WrongKind, []string{".lumilio/cache/artifacts"}) {
		t.Fatalf("Validate = %+v, %v", report, err)
	}
	if _, err := Validate(filepath.Join(root, "missing")); err == nil {
		t.Fatal("Validate accepted a missing root")
	}
}
