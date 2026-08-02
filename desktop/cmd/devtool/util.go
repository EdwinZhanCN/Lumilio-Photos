package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isDir(filepath.Join(dir, "desktop")) && isDir(filepath.Join(dir, "web")) && isDir(filepath.Join(dir, "server")) {
			return dir, nil
		}
		p := filepath.Dir(dir)
		if p == dir {
			return "", errors.New("repository root not found (expected desktop/, web/, server/)")
		}
		dir = p
	}
}
func isDir(p string) bool  { i, e := os.Stat(p); return e == nil && i.IsDir() }
func isFile(p string) bool { i, e := os.Stat(p); return e == nil && !i.IsDir() }
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func runCmd(ctx context.Context, dir string, env []string, name string, args ...string) error {
	fmt.Printf("→ %s %s\n", name, strings.Join(args, " "))
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
func outputCmd(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = dir
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	b, e := c.Output()
	if e != nil {
		if ee, ok := e.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w: %s", name, e, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, e
	}
	return b, nil
}
func requireCommand(name string) error {
	if _, e := exec.LookPath(name); e != nil {
		return fmt.Errorf("required command %q not found in PATH", name)
	}
	return nil
}
func sha256File(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func verifySHA(p, want string) error {
	got, e := sha256File(p)
	if e != nil {
		return e
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filepath.Base(p), want, got)
	}
	return nil
}
func download(ctx context.Context, url, dst string) error {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "curl/8 lumilio-devtool")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
		return e
	}
	f, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, ce := io.Copy(f, resp.Body)
	xe := f.Close()
	if ce != nil {
		return ce
	}
	return xe
}
func unzip(src, dst string) error {
	r, e := zip.OpenReader(src)
	if e != nil {
		return e
	}
	defer r.Close()
	for _, f := range r.File {
		clean := filepath.Clean(f.Name)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe zip path %q", f.Name)
		}
		out := filepath.Join(dst, clean)
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(out, 0755); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(out), 0755); e != nil {
			return e
		}
		in, e := f.Open()
		if e != nil {
			return e
		}
		mode := f.Mode()
		if mode == 0 {
			mode = 0644
		}
		o, e := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if e != nil {
			in.Close()
			return e
		}
		_, ce := io.Copy(o, in)
		in.Close()
		oe := o.Close()
		if ce != nil {
			return ce
		}
		if oe != nil {
			return oe
		}
	}
	return nil
}
func untarGz(src, dst string) error {
	f, e := os.Open(src)
	if e != nil {
		return e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		clean := filepath.Clean(h.Name)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe tar path %q", h.Name)
		}
		out := filepath.Join(dst, clean)
		switch h.Typeflag {
		case tar.TypeDir:
			e = os.MkdirAll(out, os.FileMode(h.Mode))
		case tar.TypeReg:
			if e = os.MkdirAll(filepath.Dir(out), 0755); e == nil {
				var o *os.File
				o, e = os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode))
				if e == nil {
					_, e = io.Copy(o, tr)
					o.Close()
				}
			}
		case tar.TypeSymlink:
			if e = os.MkdirAll(filepath.Dir(out), 0755); e == nil {
				e = os.Symlink(h.Linkname, out)
			}
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func copyFile(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	st, e := in.Stat()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
		return e
	}
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, st.Mode())
	if e != nil {
		return e
	}
	_, ce := io.Copy(out, in)
	xe := out.Close()
	if ce != nil {
		return ce
	}
	return xe
}
func copyTree(src, dst string) error {
	st, e := os.Stat(src)
	if e != nil {
		return e
	}
	if !st.IsDir() {
		return copyFile(src, dst)
	}
	return filepath.WalkDir(src, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			t, e := os.Readlink(p)
			if e != nil {
				return e
			}
			_ = os.Remove(out)
			return os.Symlink(t, out)
		}
		return copyFile(p, out)
	})
}
func firstFile(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), name) {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found under %s", name, root)
	}
	return found, nil
}
func commandExists(n string) bool { _, e := exec.LookPath(n); return e == nil }
func sortedUnique(xs []string) []string {
	m := map[string]struct{}{}
	for _, x := range xs {
		if x != "" {
			m[x] = struct{}{}
		}
	}
	out := make([]string, 0, len(m))
	for x := range m {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}
func executableName(n string) string {
	if runtime.GOOS == "windows" {
		switch n {
		case "vp", "npm", "pnpm", "npx":
			return n + ".cmd"
		}
	}
	return n
}
