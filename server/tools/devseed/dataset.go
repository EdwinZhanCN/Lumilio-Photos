package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	expectedScanImages        = 145
	expectedUploadImages      = 80
	expectedPicsumScanImages  = 120
	expectedLandingDemoImages = 25
)

type dataset struct {
	root        string
	scanRoot    string
	uploadRoot  string
	scanFiles   []string
	uploadFiles []string
}

type picsumManifestRecord struct {
	IngestMethod string `json:"ingest_method"`
	LocalPath    string `json:"local_path"`
	SHA256       string `json:"sha256"`
}

type landingManifestRecord struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type scanTargetInspection struct {
	Missing   int
	Matching  int
	Conflicts []string
}

func validateDataset(root string) (dataset, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return dataset{}, fmt.Errorf("resolve dataset root: %w", err)
	}
	result := dataset{
		root:       absRoot,
		scanRoot:   filepath.Join(absRoot, "media", "scan"),
		uploadRoot: filepath.Join(absRoot, "media", "upload"),
	}

	result.scanFiles, err = collectJPEGs(result.scanRoot)
	if err != nil {
		return dataset{}, fmt.Errorf("validate scan media: %w", err)
	}
	result.uploadFiles, err = collectJPEGs(result.uploadRoot)
	if err != nil {
		return dataset{}, fmt.Errorf("validate upload media: %w", err)
	}
	if len(result.scanFiles) != expectedScanImages {
		return dataset{}, fmt.Errorf("expected %d scan JPEGs, found %d", expectedScanImages, len(result.scanFiles))
	}
	if len(result.uploadFiles) != expectedUploadImages {
		return dataset{}, fmt.Errorf("expected %d upload JPEGs, found %d", expectedUploadImages, len(result.uploadFiles))
	}
	if err := validatePicsumManifest(result); err != nil {
		return dataset{}, err
	}
	if err := validateLandingManifest(result); err != nil {
		return dataset{}, err
	}
	if err := ensureUniqueContent(append(append([]string{}, result.scanFiles...), result.uploadFiles...)); err != nil {
		return dataset{}, err
	}
	return result, nil
}

func collectJPEGs(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular media file: %s", path)
		}
		if strings.ToLower(filepath.Ext(path)) != ".jpg" {
			return fmt.Errorf("unexpected non-JPEG media file: %s", path)
		}
		if err := validateJPEG(path); err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func validateJPEG(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Size() < 5_000 {
		return fmt.Errorf("JPEG is unexpectedly small (%d bytes): %s", info.Size(), path)
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("read JPEG header %s: %w", path, err)
	}
	if header[0] != 0xff || header[1] != 0xd8 {
		return fmt.Errorf("invalid JPEG header: %s", path)
	}
	return nil
}

func validatePicsumManifest(data dataset) error {
	path := filepath.Join(data.root, "manifests", "picsum-manifest.json")
	var records []picsumManifestRecord
	if err := readJSONFile(path, &records); err != nil {
		return fmt.Errorf("read Picsum manifest: %w", err)
	}
	if len(records) != expectedPicsumScanImages+expectedUploadImages {
		return fmt.Errorf("Picsum manifest has %d records, want %d", len(records), expectedPicsumScanImages+expectedUploadImages)
	}

	seen := make(map[string]struct{}, len(records))
	scanCount := 0
	uploadCount := 0
	for _, record := range records {
		var sourcePath string
		switch record.IngestMethod {
		case "scan":
			rel, ok := strings.CutPrefix(filepath.ToSlash(record.LocalPath), "scan-source/")
			if !ok {
				return fmt.Errorf("invalid scan local_path in Picsum manifest: %q", record.LocalPath)
			}
			sourcePath, ok = safeJoin(data.scanRoot, filepath.FromSlash(rel))
			if !ok {
				return fmt.Errorf("unsafe scan local_path in Picsum manifest: %q", record.LocalPath)
			}
			scanCount++
		case "upload":
			rel, ok := strings.CutPrefix(filepath.ToSlash(record.LocalPath), "upload/")
			if !ok {
				return fmt.Errorf("invalid upload local_path in Picsum manifest: %q", record.LocalPath)
			}
			sourcePath, ok = safeJoin(data.uploadRoot, filepath.FromSlash(rel))
			if !ok {
				return fmt.Errorf("unsafe upload local_path in Picsum manifest: %q", record.LocalPath)
			}
			uploadCount++
		default:
			return fmt.Errorf("unknown Picsum ingest method %q", record.IngestMethod)
		}
		if _, exists := seen[sourcePath]; exists {
			return fmt.Errorf("duplicate Picsum manifest path: %s", sourcePath)
		}
		seen[sourcePath] = struct{}{}
		if err := verifySHA256(sourcePath, record.SHA256); err != nil {
			return fmt.Errorf("Picsum manifest: %w", err)
		}
	}
	if scanCount != expectedPicsumScanImages || uploadCount != expectedUploadImages {
		return fmt.Errorf("Picsum manifest routes scan=%d upload=%d, want scan=%d upload=%d", scanCount, uploadCount, expectedPicsumScanImages, expectedUploadImages)
	}
	for _, sourcePath := range data.scanFiles {
		rel, err := filepath.Rel(data.scanRoot, sourcePath)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), "landing-demo/") {
			continue
		}
		if _, exists := seen[sourcePath]; !exists {
			return fmt.Errorf("scan JPEG missing from Picsum manifest: %s", sourcePath)
		}
	}
	for _, sourcePath := range data.uploadFiles {
		if _, exists := seen[sourcePath]; !exists {
			return fmt.Errorf("upload JPEG missing from Picsum manifest: %s", sourcePath)
		}
	}
	return nil
}

func validateLandingManifest(data dataset) error {
	path := filepath.Join(data.root, "manifests", "landing-demo-manifest.json")
	var records []landingManifestRecord
	if err := readJSONFile(path, &records); err != nil {
		return fmt.Errorf("read landing-demo manifest: %w", err)
	}
	if len(records) != expectedLandingDemoImages {
		return fmt.Errorf("landing-demo manifest has %d records, want %d", len(records), expectedLandingDemoImages)
	}

	landingRoot := filepath.Join(data.scanRoot, "landing-demo")
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if filepath.Base(record.File) != record.File || strings.TrimSpace(record.File) == "" {
			return fmt.Errorf("invalid landing-demo manifest filename %q", record.File)
		}
		sourcePath := filepath.Join(landingRoot, record.File)
		if _, exists := seen[sourcePath]; exists {
			return fmt.Errorf("duplicate landing-demo manifest path: %s", record.File)
		}
		seen[sourcePath] = struct{}{}
		if err := verifySHA256(sourcePath, record.SHA256); err != nil {
			return fmt.Errorf("landing-demo manifest: %w", err)
		}
	}
	return nil
}

func readJSONFile(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func verifySHA256(path, expected string) error {
	actual, err := sha256File(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(expected), actual) {
		return fmt.Errorf("SHA-256 mismatch for %s: got %s want %s", path, actual, expected)
	}
	return nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func ensureUniqueContent(paths []string) error {
	seen := make(map[string]string, len(paths))
	for _, path := range paths {
		digest, err := sha256File(path)
		if err != nil {
			return err
		}
		if previous, exists := seen[digest]; exists {
			return fmt.Errorf("duplicate seed bytes: %s and %s", previous, path)
		}
		seen[digest] = path
	}
	return nil
}

func inspectScanTargets(data dataset, repositoryPath string) (scanTargetInspection, error) {
	inspection := scanTargetInspection{}
	for _, sourcePath := range data.scanFiles {
		rel, err := filepath.Rel(data.scanRoot, sourcePath)
		if err != nil {
			return inspection, err
		}
		targetPath, ok := safeJoin(repositoryPath, rel)
		if !ok {
			return inspection, fmt.Errorf("unsafe scan target path: %s", rel)
		}
		match, exists, err := filesMatch(sourcePath, targetPath)
		if err != nil {
			return inspection, err
		}
		switch {
		case !exists:
			inspection.Missing++
		case match:
			inspection.Matching++
		default:
			inspection.Conflicts = append(inspection.Conflicts, filepath.ToSlash(rel))
		}
	}
	return inspection, nil
}

func materializeScanFiles(data dataset, repositoryPath string) (copied, retained int, err error) {
	for _, sourcePath := range data.scanFiles {
		rel, relErr := filepath.Rel(data.scanRoot, sourcePath)
		if relErr != nil {
			return copied, retained, relErr
		}
		targetPath, ok := safeJoin(repositoryPath, rel)
		if !ok {
			return copied, retained, fmt.Errorf("unsafe scan target path: %s", rel)
		}
		match, exists, matchErr := filesMatch(sourcePath, targetPath)
		if matchErr != nil {
			return copied, retained, matchErr
		}
		if exists {
			if !match {
				return copied, retained, fmt.Errorf("refusing to overwrite conflicting scan file: %s", targetPath)
			}
			retained++
			continue
		}
		if copyErr := copyFileExclusive(sourcePath, targetPath); copyErr != nil {
			return copied, retained, copyErr
		}
		copied++
	}
	return copied, retained, nil
}

func filesMatch(sourcePath, targetPath string) (match, exists bool, err error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, false, err
	}
	targetInfo, err := os.Stat(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if !targetInfo.Mode().IsRegular() || sourceInfo.Size() != targetInfo.Size() {
		return false, true, nil
	}
	sourceHash, err := sha256File(sourcePath)
	if err != nil {
		return false, true, err
	}
	targetHash, err := sha256File(targetPath)
	if err != nil {
		return false, true, err
	}
	return sourceHash == targetHash, true, nil
}

func copyFileExclusive(sourcePath, targetPath string) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create scan folder for %s: %w", targetPath, err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	sourceInfo, err := source.Stat()
	if err != nil {
		return err
	}

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create scan file %s: %w", targetPath, err)
	}
	defer func() {
		if closeErr := target.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
		if returnErr != nil {
			_ = os.Remove(targetPath)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy scan file %s: %w", targetPath, err)
	}
	if err := target.Sync(); err != nil {
		return fmt.Errorf("sync scan file %s: %w", targetPath, err)
	}
	if err := os.Chtimes(targetPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("preserve scan file time %s: %w", targetPath, err)
	}
	return nil
}

func safeJoin(root, rel string) (string, bool) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(rel) == "" || filepath.IsAbs(rel) {
		return "", false
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", false
	}
	target, err := filepath.Abs(filepath.Join(cleanRoot, filepath.Clean(rel)))
	if err != nil {
		return "", false
	}
	relToRoot, err := filepath.Rel(cleanRoot, target)
	if err != nil || relToRoot == "." || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", false
	}
	return target, true
}
