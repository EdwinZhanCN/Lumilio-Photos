package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"sync"
	"testing"
)

// synthJPEG renders a deterministic w*h gradient as a JPEG buffer. Used to
// drive imaging tests without checking a binary fixture into the repo.
func synthJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode synth jpeg: %v", err)
	}
	return buf.Bytes()
}

func runStreamThumbnails(src []byte, sizes map[string][2]int) (map[string][]byte, error) {
	bufs := make(map[string]*bytes.Buffer, len(sizes))
	writers := make(map[string]io.Writer, len(sizes))
	for name := range sizes {
		b := &bytes.Buffer{}
		bufs[name] = b
		writers[name] = b
	}
	if err := StreamThumbnails(bytes.NewReader(src), sizes, writers); err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(bufs))
	for name, b := range bufs {
		out[name] = append([]byte(nil), b.Bytes()...)
	}
	return out, nil
}

func TestStreamThumbnails_BasicOutput(t *testing.T) {
	StartVips()

	src := synthJPEG(t, 1024, 768)
	sizes := map[string][2]int{
		"small":  {400, 400},
		"medium": {800, 800},
		"large":  {1920, 1920},
	}

	out, err := runStreamThumbnails(src, sizes)
	if err != nil {
		t.Fatalf("StreamThumbnails: %v", err)
	}

	for name, b := range out {
		if len(b) == 0 {
			t.Fatalf("size %q: empty output", name)
		}
		// WebP magic: "RIFF" + 4-byte size + "WEBP".
		if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
			t.Fatalf("size %q: output is not WebP (head=% x)", name, b[:min(16, len(b))])
		}
	}
}

// TestStreamThumbnails_Concurrent exercises the same pipeline from many
// goroutines at once. This is the regression guard for the libvips/libexif
// concurrency race that previously produced corrupted ("rainbow stripe") WebP
// output. With govips Startup(ConcurrencyLevel: 1) configured in StartVips,
// outputs must remain byte-for-byte stable across runs of the same input.
func TestStreamThumbnails_Concurrent(t *testing.T) {
	StartVips()

	src := synthJPEG(t, 640, 480)
	sizes := map[string][2]int{
		"small":  {200, 200},
		"medium": {400, 400},
	}

	ref, err := runStreamThumbnails(src, sizes)
	if err != nil {
		t.Fatalf("reference run: %v", err)
	}

	const goroutines = 16
	const iterations = 8

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				got, err := runStreamThumbnails(src, sizes)
				if err != nil {
					errs <- fmt.Errorf("goroutine %d iter %d: %w", gid, i, err)
					return
				}
				for name, b := range got {
					if !bytes.Equal(ref[name], b) {
						errs <- fmt.Errorf("goroutine %d iter %d size %q: byte mismatch (ref=%d got=%d)", gid, i, name, len(ref[name]), len(b))
						return
					}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent run produced divergent output: %v", err)
	}
}
