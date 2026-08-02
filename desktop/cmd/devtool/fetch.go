package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func fetchResources(ctx context.Context, root string, args []string) error {
	switch runtime.GOOS {
	case "darwin":
		return fetchMacResources(ctx, root)
	case "windows":
		return fetchWindowsResources(ctx, root)
	default:
		return fmt.Errorf("fetch-resources supports macOS and Windows; current OS is %s", runtime.GOOS)
	}
}
func fetchMacResources(ctx context.Context, root string) error {
	res := filepath.Join(root, "desktop", "resources")
	tmp, err := os.MkdirTemp("", "lumilio-resources-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	ff := []struct{ name, url, sha string }{{"ffmpeg", envOr("FFMPEG_URL", "https://www.osxexperts.net/ffmpeg81arm.zip"), envOr("FFMPEG_SHA256", "9a08d61f9328e8164ba560ee7a79958e357307fcfeea6fe626b7d66cdc287028")}, {"ffprobe", envOr("FFPROBE_URL", "https://www.osxexperts.net/ffprobe81arm.zip"), envOr("FFPROBE_SHA256", "aab17ac7379c1178aaf400c3ef36cdb67db0b75b1a23eeef2cb9f658be8844e6")}}
	fmt.Printf("==> Fetching bundled media tools into %s\n", res)
	for _, x := range ff {
		dst := filepath.Join(res, "ffmpeg", x.name)
		if isFile(dst) {
			if got, _ := sha256File(dst); strings.EqualFold(got, x.sha) {
				fmt.Printf("  %s: already present and verified — skipping\n", x.name)
				continue
			}
		}
		work := filepath.Join(tmp, x.name)
		os.MkdirAll(work, 0755)
		zipPath := filepath.Join(work, "payload.zip")
		fmt.Printf("  %s: downloading %s\n", x.name, x.url)
		if err := download(ctx, x.url, zipPath); err != nil {
			return err
		}
		if err := unzip(zipPath, work); err != nil {
			return err
		}
		src, err := firstFile(work, x.name)
		if err != nil {
			return err
		}
		if err := verifySHA(src, x.sha); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
		os.Chmod(dst, 0755)
		_ = runCmd(ctx, root, nil, "xattr", "-dr", "com.apple.quarantine", dst)
		got, _ := sha256File(dst)
		fmt.Printf("  %s: installed → ffmpeg/%s (%s…)\n", x.name, x.name, got[:12])
	}
	ver := envOr("EXIFTOOL_VERSION", "13.59")
	url := envOr("EXIFTOOL_URL", "https://downloads.sourceforge.net/project/exiftool/Image-ExifTool-"+ver+".tar.gz")
	sha := envOr("EXIFTOOL_SHA256", "668ea3acececb7235fbd0f4900e72d5f12c9b07e5c778fd36cb1e9b5828fd65a")
	dest := filepath.Join(res, "exiftool")
	if isFile(filepath.Join(dest, "exiftool")) && isDir(filepath.Join(dest, "lib")) {
		fmt.Println("  exiftool: already present (exiftool + lib/) — skipping")
	} else {
		arc := filepath.Join(tmp, "exiftool.tar.gz")
		fmt.Printf("  exiftool: downloading %s\n", url)
		if err := download(ctx, url, arc); err != nil {
			return err
		}
		if err := verifySHA(arc, sha); err != nil {
			return err
		}
		work := filepath.Join(tmp, "exiftool")
		os.MkdirAll(work, 0755)
		if err := untarGz(arc, work); err != nil {
			return err
		}
		ents, _ := os.ReadDir(work)
		var src string
		for _, e := range ents {
			if e.IsDir() && strings.HasPrefix(e.Name(), "Image-ExifTool-") {
				src = filepath.Join(work, e.Name())
				break
			}
		}
		if src == "" || !isFile(filepath.Join(src, "exiftool")) || !isDir(filepath.Join(src, "lib")) {
			return fmt.Errorf("extracted exiftool tree not found")
		}
		os.RemoveAll(dest)
		os.MkdirAll(dest, 0755)
		if err := copyFile(filepath.Join(src, "exiftool"), filepath.Join(dest, "exiftool")); err != nil {
			return err
		}
		if err := copyTree(filepath.Join(src, "lib"), filepath.Join(dest, "lib")); err != nil {
			return err
		}
		os.Chmod(filepath.Join(dest, "exiftool"), 0755)
		_ = runCmd(ctx, root, nil, "xattr", "-dr", "com.apple.quarantine", dest)
		fmt.Printf("  exiftool: installed → exiftool/{exiftool,lib} (v%s)\n", ver)
	}
	fmt.Println("==> Done.")
	return nil
}
func fetchWindowsResources(ctx context.Context, root string) error {
	res := filepath.Join(root, "desktop", "resources")
	work := filepath.Join(res, ".downloads")
	os.RemoveAll(work)
	os.MkdirAll(work, 0755)
	defer os.RemoveAll(work)
	ffURL := envOr("FFMPEG_URL", "https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-8.1.2-essentials_build.zip")
	ffSHA := envOr("FFMPEG_SHA256", "db580001caa24ac104c8cb856cd113a87b0a443f7bdf47d8c12b1d740584a2ec")
	ffzip := filepath.Join(work, "ffmpeg.zip")
	fmt.Printf("==> Downloading %s\n", ffURL)
	if err := download(ctx, ffURL, ffzip); err != nil {
		return err
	}
	if err := verifySHA(ffzip, ffSHA); err != nil {
		return err
	}
	ffdir := filepath.Join(work, "ffmpeg")
	if err := unzip(ffzip, ffdir); err != nil {
		return err
	}
	ffmpeg, err := firstFile(ffdir, "ffmpeg.exe")
	if err != nil {
		return err
	}
	ffprobe := filepath.Join(filepath.Dir(ffmpeg), "ffprobe.exe")
	if !isFile(ffprobe) {
		return fmt.Errorf("ffprobe.exe not found beside ffmpeg.exe")
	}
	dest := filepath.Join(res, "ffmpeg")
	os.MkdirAll(dest, 0755)
	copyFile(ffmpeg, filepath.Join(dest, "ffmpeg.exe"))
	copyFile(ffprobe, filepath.Join(dest, "ffprobe.exe"))
	fmt.Printf("==> Staged %s\n", dest)
	etURL := envOr("EXIFTOOL_URL", "https://downloads.sourceforge.net/project/exiftool/exiftool-13.59_64.zip")
	etSHA := envOr("EXIFTOOL_SHA256", "44b512b25af500724ba579d0a53c8fc5851628b692dd5e5d94ae4a15c2cba9ec")
	zipP := filepath.Join(work, "exiftool.zip")
	fmt.Printf("==> Downloading %s\n", etURL)
	if err := download(ctx, etURL, zipP); err != nil {
		return err
	}
	if err := verifySHA(zipP, etSHA); err != nil {
		return err
	}
	etdir := filepath.Join(work, "exiftool")
	if err := unzip(zipP, etdir); err != nil {
		return err
	}
	exe, err := firstFile(etdir, "exiftool(-k).exe")
	if err != nil {
		return err
	}
	edest := filepath.Join(res, "exiftool")
	os.RemoveAll(edest)
	os.MkdirAll(edest, 0755)
	if err := copyFile(exe, filepath.Join(edest, "exiftool.exe")); err != nil {
		return err
	}
	files := filepath.Join(filepath.Dir(exe), "exiftool_files")
	if isDir(files) {
		if err := copyTree(files, filepath.Join(edest, "exiftool_files")); err != nil {
			return err
		}
	}
	fmt.Printf("==> Staged %s\n==> Done\n", edest)
	return nil
}
