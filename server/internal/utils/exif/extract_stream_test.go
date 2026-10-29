package exif

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("LUMILIO_EXIF_TEST_PROCESS"); mode != "" {
		if mode == "read" {
			_, _ = io.Copy(io.Discard, os.Stdin)
		}
		if mode == "wait" {
			time.Sleep(time.Minute)
		}
		_ = os.Stdin.Close()
		if mode == "invalid" {
			fmt.Print("invalid JSON")
		} else {
			fmt.Print(`[{"FileType":"JXL Codestream","ImageWidth":1440,"ImageHeight":1080}]`)
		}
		if mode == "fail" {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func streamTestExtractor(t *testing.T, mode string) *Extractor {
	t.Helper()
	t.Setenv("LUMILIO_EXIF_TEST_PROCESS", mode)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.ExifToolPath = executable
	cfg.BufferSize = 128 * 1024
	cfg.Timeout = 5 * time.Second
	return NewExtractor(cfg)
}

func TestStreamEarlySuccessfulInputClose(t *testing.T) {
	e := streamTestExtractor(t, "early")
	metadata, raw, err := e.runExifToolFromStream(context.Background(), bytes.NewReader(make([]byte, 4*1024*1024)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if metadata["ImageWidth"] != "1440" || !strings.Contains(string(raw), "JXL Codestream") {
		t.Fatalf("metadata missing: %v %s", metadata, raw)
	}
}

func TestStreamRejectsProcessAndOutputErrors(t *testing.T) {
	for _, mode := range []string{"fail", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			e := streamTestExtractor(t, mode)
			if _, _, err := e.runExifToolFromStream(context.Background(), strings.NewReader("input"), nil); err == nil {
				t.Fatal("accepted failed extraction")
			}
		})
	}
}

type failingMetadataReader struct{ err error }

func (r failingMetadataReader) Read([]byte) (int, error) { return 0, r.err }

func TestStreamPreservesSourceErrors(t *testing.T) {
	for _, sourceErr := range []error{errors.New("source unavailable"), syscall.EPIPE} {
		e := streamTestExtractor(t, "read")
		_, _, err := e.runExifToolFromStream(context.Background(), failingMetadataReader{sourceErr}, nil)
		if !errors.Is(err, sourceErr) {
			t.Fatalf("lost source error %v: %v", sourceErr, err)
		}
	}
}

func TestStreamCancellation(t *testing.T) {
	e := streamTestExtractor(t, "wait")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, _, err := e.runExifToolFromStream(ctx, strings.NewReader("input"), nil); err == nil {
		t.Fatal("accepted canceled extraction")
	}
}
