package processors

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"server/config"
)

func TestAudioArtworkExtractsEmbeddedImageAndAllowsMissingCover(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available, skipping audio artwork extraction tests")
	}
	dir := t.TempDir()
	art := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			art.Set(x, y, color.RGBA{R: 210, G: 40, B: 60, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, art))
	cover := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(cover, buf.Bytes(), 0600))
	plain := filepath.Join(dir, "plain.mp3")
	out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-y", plain).CombinedOutput()
	require.NoError(t, err, string(out))
	embedded := filepath.Join(dir, "embedded.mp3")
	out, err = exec.Command(ffmpeg, "-v", "error", "-i", plain, "-i", cover, "-map", "0:a", "-map", "1:v", "-c", "copy", "-disposition:v", "attached_pic", "-y", embedded).CombinedOutput()
	require.NoError(t, err, string(out))
	ap := &AssetProcessor{toolsConfig: config.ToolsConfig{FFmpegPath: ffmpeg}}
	result, err := ap.extractAudioArtwork(context.Background(), embedded)
	require.NoError(t, err)
	decoded, err := png.Decode(bytes.NewReader(result))
	require.NoError(t, err)
	r, g, b, _ := decoded.At(0, 0).RGBA()
	require.Greater(t, r, g)
	require.Greater(t, r, b)
	result, err = ap.extractAudioArtwork(context.Background(), plain)
	require.NoError(t, err)
	require.Empty(t, result)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ap.extractAudioArtwork(ctx, embedded)
	require.ErrorIs(t, err, context.Canceled)
}
