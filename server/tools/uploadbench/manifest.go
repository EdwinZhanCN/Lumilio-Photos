package main

import (
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"server/internal/utils/hash"
)

// fileRec is one dataset entry plus every timing captured for it during a run.
// All *Ns fields are Unix-nanosecond timestamps (0 = not observed).
type fileRec struct {
	// Immutable manifest fields.
	Path string `json:"path"`
	Name string `json:"name"`
	Ext  string `json:"ext"`
	Size int64  `json:"size"`
	MIME string `json:"mime"`
	Hash string `json:"hash,omitempty"`

	// Upload phase.
	UploadStartNs int64  `json:"upload_start_ns,omitempty"`
	UploadEndNs   int64  `json:"upload_end_ns,omitempty"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	TaskID        int64  `json:"task_id,omitempty"`
	UploadErr     string `json:"upload_err,omitempty"`

	// Completion phase (first observation that both core tasks are complete).
	AssetID    string `json:"asset_id,omitempty"`
	CompleteNs int64  `json:"complete_ns,omitempty"`
	Failed     bool   `json:"failed,omitempty"` // a core task reached "failed"
}

// manifest is the immutable description of the dataset used for a run.
type manifest struct {
	Dataset     string     `json:"dataset"`
	GeneratedAt time.Time  `json:"generated_at"`
	Exts        []string   `json:"exts"`
	FileCount   int        `json:"file_count"`
	TotalBytes  int64      `json:"total_bytes"`
	ClientHash  bool       `json:"client_hash"`
	Files       []*fileRec `json:"files"`
}

// buildManifest walks the dataset directory, keeps files whose extension is in
// exts, and (when clientHash) computes the BLAKE3 hash exactly as the server
// would, so client-provided X-Content-Hash matches the stored asset hash.
func buildManifest(cfg config) (*manifest, error) {
	allow := make(map[string]bool, len(cfg.exts))
	for _, e := range cfg.exts {
		allow[e] = true
	}

	var files []*fileRec
	err := filepath.WalkDir(cfg.dataset, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if !allow[ext] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mimeType := mime.TypeByExtension("." + ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		files = append(files, &fileRec{
			Path: path,
			Name: filepath.Base(path),
			Ext:  ext,
			Size: info.Size(),
			MIME: mimeType,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dataset: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files matching exts %v under %s", cfg.exts, cfg.dataset)
	}

	// Deterministic order, then apply the optional cap.
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	if cfg.limit > 0 && cfg.limit < len(files) {
		files = files[:cfg.limit]
	}

	// Reject duplicate filenames: the poller matches assets by original
	// filename, so a collision would make completion attribution ambiguous.
	seen := make(map[string]string, len(files))
	for _, f := range files {
		if prev, ok := seen[f.Name]; ok {
			return nil, fmt.Errorf("duplicate filename %q (%s and %s); dataset hygiene requires unique names", f.Name, prev, f.Path)
		}
		seen[f.Name] = f.Path
	}

	if cfg.clientHash {
		if err := hashFiles(files, cfg.concurrency); err != nil {
			return nil, err
		}
	}

	var total int64
	for _, f := range files {
		total += f.Size
	}
	return &manifest{
		Dataset:     cfg.dataset,
		GeneratedAt: time.Now(),
		Exts:        cfg.exts,
		FileCount:   len(files),
		TotalBytes:  total,
		ClientHash:  cfg.clientHash,
		Files:       files,
	}, nil
}

// hashFiles fills f.Hash for every file, using the same BLAKE3 routine the
// server uses so the trusted client hash matches the stored asset hash.
func hashFiles(files []*fileRec, workers int) error {
	log.Printf("hashing %d files (BLAKE3, %d workers)...", len(files), workers)
	start := time.Now()

	jobs := make(chan *fileRec)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				res, err := hash.CalculateFileHash(f.Path, hash.AlgorithmBLAKE3, true)
				if err != nil {
					errs <- fmt.Errorf("hash %s: %w", f.Name, err)
					return
				}
				f.Hash = res.Hash
			}
		}()
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		return err
	}
	log.Printf("hashed %d files in %s", len(files), time.Since(start).Round(time.Millisecond))
	return nil
}
