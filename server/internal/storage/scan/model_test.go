package scan

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const modelSeeds = 64

// TestModelCatalogConvergesToDisk applies random tree changes (create,
// modify, delete, file rename, directory rename, duplicate copy, case-only
// rename) while scans and hash passes are interrupted at random points.
// After one uninterrupted full scan and hash drain, the catalog must mirror
// the disk exactly and no Asset may be left without an entry.
func TestModelCatalogConvergesToDisk(t *testing.T) {
	for seed := int64(1); seed <= modelSeeds; seed++ {
		t.Run("seed-"+strconv.FormatInt(seed, 10), func(t *testing.T) {
			runModel(t, seed)
		})
	}
}

type model struct {
	f      *fixture
	random *rand.Rand
	next   int
	pool   []string
}

func runModel(t *testing.T, seed int64) {
	f := newFixture(t, 0)
	f.scanner.config.TurnFiles = 3
	m := &model{f: f, random: rand.New(rand.NewSource(seed))}
	for index := 0; index < 8; index++ {
		m.create()
	}
	f.scan(f.primary)
	f.hash(f.primary)
	for step := 0; step < 40; step++ {
		switch m.random.Intn(9) {
		case 0, 1:
			m.create()
		case 2:
			m.modify()
		case 3:
			m.deleteFile()
		case 4:
			m.renameFile()
		case 5:
			m.renameDirectory()
		case 6:
			m.copyFile()
		case 7:
			m.caseRename()
		case 8:
			m.interrupt()
		}
	}
	m.finishOutstanding()
	f.scan(f.primary)
	f.hash(f.primary)
	m.assertMirrorsDisk()
}

func (m *model) files() []string {
	var out []string
	root := m.f.repos[m.f.primary.RepoID]
	_ = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, name)
		relative = filepath.ToSlash(relative)
		if entry.IsDir() && relative == ".lumilio" {
			return fs.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(relative, ".jpg") {
			out = append(out, relative)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (m *model) directories() []string {
	var out []string
	root := m.f.repos[m.f.primary.RepoID]
	_ = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		relative, _ := filepath.Rel(root, name)
		relative = filepath.ToSlash(relative)
		if relative == ".lumilio" {
			return fs.SkipDir
		}
		if relative != "." {
			out = append(out, relative)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (m *model) pick(items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}
	return items[m.random.Intn(len(items))], true
}

// content reuses earlier content a third of the time, so copies and edits
// that reproduce another Asset's bytes happen.
func (m *model) content() string {
	if len(m.pool) > 0 && m.random.Intn(3) == 0 {
		return m.pool[m.random.Intn(len(m.pool))]
	}
	m.next++
	value := "content-" + strconv.Itoa(m.next) + strings.Repeat("x", m.random.Intn(64))
	m.pool = append(m.pool, value)
	return value
}

func (m *model) freshName() string {
	m.next++
	return "f" + strconv.Itoa(m.next) + ".jpg"
}

func (m *model) directory() string {
	candidates := append([]string{""}, m.directories()...)
	if m.random.Intn(4) == 0 {
		m.next++
		base, _ := m.pick(candidates)
		return strings.TrimPrefix(base+"/d"+strconv.Itoa(m.next), "/")
	}
	choice, _ := m.pick(candidates)
	return choice
}

func (m *model) create() {
	m.f.write(m.f.primary, strings.TrimPrefix(m.directory()+"/"+m.freshName(), "/"), m.content())
}

func (m *model) modify() {
	if target, ok := m.pick(m.files()); ok {
		m.f.write(m.f.primary, target, m.content())
	}
}

func (m *model) deleteFile() {
	if target, ok := m.pick(m.files()); ok {
		m.f.remove(m.f.primary, target)
	}
}

func (m *model) renameFile() {
	if source, ok := m.pick(m.files()); ok {
		m.f.rename(m.f.primary, source, m.f.primary, strings.TrimPrefix(m.directory()+"/"+m.freshName(), "/"))
	}
}

func (m *model) renameDirectory() {
	source, ok := m.pick(m.directories())
	if !ok {
		return
	}
	m.next++
	target := strings.TrimPrefix(parentOf(source)+"/r"+strconv.Itoa(m.next), "/")
	m.f.rename(m.f.primary, source, m.f.primary, target)
}

func (m *model) copyFile() {
	source, ok := m.pick(m.files())
	if !ok {
		return
	}
	data, err := os.ReadFile(m.f.path(m.f.primary, source))
	if err != nil {
		m.f.t.Fatal(err)
	}
	m.f.write(m.f.primary, strings.TrimPrefix(m.directory()+"/"+m.freshName(), "/"), string(data))
}

// caseRename changes only the case of a file name. On a case-insensitive
// volume the entry keeps its key; elsewhere it is an ordinary rename.
func (m *model) caseRename() {
	source, ok := m.pick(m.files())
	if !ok {
		return
	}
	directory, name := parentOf(source), source[strings.LastIndexByte(source, '/')+1:]
	renamed := strings.ToUpper(name[:1]) + name[1:]
	if renamed == name {
		renamed = strings.ToLower(name[:1]) + name[1:]
	}
	m.f.rename(m.f.primary, source, m.f.primary, strings.TrimPrefix(directory+"/"+renamed, "/"))
}

// interrupt starts a scan and abandons it after a few turns, optionally
// hashing part of the backlog, so later disk changes land mid-scan.
func (m *model) interrupt() {
	f := m.f
	if _, _, err := f.scanner.Request(f.ctx, f.primary.RepoID, TriggerWatcher, "", "", time.Time{}); err != nil {
		f.t.Fatal(err)
	}
	for turns := m.random.Intn(4); turns > 0; turns-- {
		if _, err := f.scanner.RunTurn(f.ctx, f.primary.RepoID); err != nil {
			f.t.Fatal(err)
		}
	}
	if m.random.Intn(2) == 0 {
		if _, err := f.scanner.HashTurn(f.ctx, f.primary.RepoID, 1+m.random.Intn(3)); err != nil {
			f.t.Fatal(err)
		}
	}
}

// finishOutstanding completes whatever scans the interruptions left behind.
func (m *model) finishOutstanding() {
	f := m.f
	for turn := 0; turn < 100_000; turn++ {
		result, err := f.scanner.RunTurn(f.ctx, f.primary.RepoID)
		if err != nil {
			f.t.Fatal(err)
		}
		if result.ScanID == uuid.Nil {
			return
		}
	}
	f.t.Fatal("outstanding scans did not finish")
}

func (m *model) assertMirrorsDisk() {
	f := m.f
	t := f.t
	liveFiles := map[string]bool{}
	liveDirectories := map[string]bool{}
	for _, row := range f.entries(f.primary) {
		switch {
		case row.State == StatePendingHash:
			t.Fatalf("entry %s is still pending after the hash drain", row.Path)
		case row.State == StatePresent && row.Kind == KindFile:
			if liveFiles[row.Path] {
				t.Fatalf("two live entries at %s", row.Path)
			}
			liveFiles[row.Path] = true
			if got, want := f.assetContentHash(row.AssetID.UUID), fileHash(t, f.path(f.primary, row.Path)); got != want {
				t.Fatalf("%s is bound to content %s, disk holds %s", row.Path, got, want)
			}
		case row.State == StatePresent:
			liveDirectories[row.Path] = true
		}
	}
	disk := m.files()
	if len(disk) != len(liveFiles) {
		t.Fatalf("disk files %v, catalog files %v", disk, liveFiles)
	}
	for _, relative := range disk {
		if !liveFiles[relative] {
			t.Fatalf("disk file %s has no present entry; catalog %v", relative, f.entries(f.primary))
		}
	}
	directories := m.directories()
	if len(directories) != len(liveDirectories) {
		t.Fatalf("disk directories %v, catalog directories %v", directories, liveDirectories)
	}
	for _, relative := range directories {
		if !liveDirectories[relative] {
			t.Fatalf("disk directory %s has no present entry", relative)
		}
	}
	var orphans int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT count(*) FROM assets
		WHERE NOT EXISTS (SELECT 1 FROM repository_entries entry WHERE entry.asset_id = assets.asset_id)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Fatalf("%d Assets have no entry", orphans)
	}
}
