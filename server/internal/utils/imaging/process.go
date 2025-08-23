package imaging

import (
	"fmt"
	"io"
	"sync"

	"github.com/h2non/bimg"
)

// ProcessImageStream reads an image from the provided io.Reader, processes it using bimg
func ProcessImageStream(r io.Reader, opts bimg.Options) ([]byte, error) {
	// Read entire input into buffer
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Initialize bimg image
	img := bimg.NewImage(buf)

	// Perform processing with given options
	newBuf, err := img.Process(opts)
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func StreamThumbnails(
	r io.Reader,
	sizes map[string][2]int,
	outputs map[string]io.Writer,
) error {
	srcBuf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read source image: %w", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(sizes))

	for name, dim := range sizes {
		w, ok := outputs[name]
		if !ok {
			return fmt.Errorf("missing writer for size %q", name)
		}

		wg.Add(1)
		go func(name string, w io.Writer, width, height int) {
			defer wg.Done()

			opts := bimg.Options{
				Width:   width,
				Height:  height,
				Crop:    true,
				Quality: 80,
				Type:    bimg.WEBP,
			}
			thumb, err := bimg.NewImage(srcBuf).Process(opts)
			if err != nil {
				errCh <- fmt.Errorf("[%s] process: %w", name, err)
				return
			}
			if _, err := w.Write(thumb); err != nil {
				errCh <- fmt.Errorf("[%s] write: %w", name, err)
			}
		}(name, w, dim[0], dim[1])
	}

	wg.Wait()
	close(errCh)

	if err, ok := <-errCh; ok {
		return err
	}
	return nil
}

// ProcessImageBytes processes raw image bytes with bimg options and returns the result.
func ProcessImageBytes(buf []byte, opts bimg.Options) ([]byte, error) {
	img := bimg.NewImage(buf)
	return img.Process(opts)
}
