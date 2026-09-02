package materializer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"server/internal/storage/roe/corpus"
	hashutil "server/internal/utils/hash"
)

const (
	controlledHashFileBytes  int64 = 64 << 20
	controlledHashWorkers          = 4
	controlledHashProfileEnv       = "LUMILIO_ROE_HASH_PROFILE"
)

type writeOnlyDiscard struct{}

func (writeOnlyDiscard) Write(payload []byte) (int, error) { return len(payload), nil }

type profileReaderOnly struct{ io.Reader }

func TestControlledFullHashThroughputTracksSequentialRead(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled full-hash throughput profile")
	}
	if os.Getenv(controlledHashProfileEnv) != "1" {
		t.Skipf("set %s=1 to run the isolated physical-volume throughput profile", controlledHashProfileEnv)
	}
	directory := t.TempDir()
	filenames := make([]string, 0, controlledHashWorkers)
	writeCacheControlled := true
	for worker := range controlledHashWorkers {
		filename := filepath.Join(directory, "controlled-content-"+string(rune('a'+worker))+".bin")
		controlled, err := writeControlledHashFile(filename, byte(0x5a+worker))
		if err != nil {
			t.Fatal(err)
		}
		writeCacheControlled = writeCacheControlled && controlled
		filenames = append(filenames, filename)
	}

	// Warm both paths before alternating measurements so page-cache state is
	// shared rather than giving either implementation an artificial cold-read
	// advantage. Four workers match the bounded production hash queue.
	if _, _, err := measureSequentialReadSet(context.Background(), filenames); err != nil {
		t.Fatal(err)
	}
	if _, _, err := measureConcurrentFullHash(context.Background(), filenames); err != nil {
		t.Fatal(err)
	}

	readDurations := make([]time.Duration, 0, 3)
	hashDurations := make([]time.Duration, 0, 3)
	cacheControlled := writeCacheControlled
	for range 3 {
		hashDuration, hashControlled, err := measureConcurrentFullHash(context.Background(), filenames)
		if err != nil {
			t.Fatal(err)
		}
		readDuration, readControlled, err := measureSequentialReadSet(context.Background(), filenames)
		if err != nil {
			t.Fatal(err)
		}
		hashDurations = append(hashDurations, hashDuration)
		readDurations = append(readDurations, readDuration)
		cacheControlled = cacheControlled && hashControlled && readControlled
	}
	readMedian := medianDuration(readDurations)
	hashMedian := medianDuration(hashDurations)
	ratio := float64(readMedian) / float64(hashMedian)
	totalBytes := controlledHashFileBytes * controlledHashWorkers
	readMiBPerSecond := float64(totalBytes) / (1 << 20) / readMedian.Seconds()
	hashMiBPerSecond := float64(totalBytes) / (1 << 20) / hashMedian.Seconds()
	t.Logf(
		"controlled full hash: %.1f MiB/s; sequential read: %.1f MiB/s; ratio: %.1f%%",
		hashMiBPerSecond,
		readMiBPerSecond,
		ratio*100,
	)
	if !cacheControlled {
		t.Log("native cache bypass is unavailable; recording throughput without enforcing the physical-volume ratio")
		return
	}
	if ratio < 0.70 {
		t.Fatalf("full-hash throughput ratio = %.1f%%, want >=70%%", ratio*100)
	}
}

func writeControlledHashFile(filename string, seed byte) (bool, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, err
	}
	cacheControlled, prepareErr := prepareControlledProfileRead(file)
	if prepareErr != nil {
		_ = file.Close()
		return cacheControlled, prepareErr
	}
	written, copyErr := io.CopyBuffer(file, corpus.ByteReader(controlledHashFileBytes, seed), make([]byte, 1<<20))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		return cacheControlled, errors.Join(copyErr, syncErr, closeErr)
	}
	if written != controlledHashFileBytes {
		return cacheControlled, io.ErrShortWrite
	}
	return cacheControlled, nil
}

func measureSequentialReadSet(ctx context.Context, filenames []string) (time.Duration, bool, error) {
	started := time.Now()
	cacheControlled := true
	for _, filename := range filenames {
		_, controlled, err := measureSequentialRead(ctx, filename)
		cacheControlled = cacheControlled && controlled
		if err != nil {
			return 0, cacheControlled, err
		}
	}
	return time.Since(started), cacheControlled, nil
}

func measureConcurrentFullHash(ctx context.Context, filenames []string) (time.Duration, bool, error) {
	type measurement struct {
		controlled bool
		err        error
	}
	started := time.Now()
	results := make(chan measurement, len(filenames))
	for _, filename := range filenames {
		filename := filename
		go func() {
			_, controlled, err := measureFullHash(ctx, filename)
			results <- measurement{controlled: controlled, err: err}
		}()
	}
	cacheControlled := true
	for range filenames {
		result := <-results
		cacheControlled = cacheControlled && result.controlled
		if result.err != nil {
			return 0, cacheControlled, result.err
		}
	}
	return time.Since(started), cacheControlled, nil
}

func measureSequentialRead(ctx context.Context, filename string) (time.Duration, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	file, err := os.Open(filename)
	if err != nil {
		return 0, false, err
	}
	cacheControlled, prepareErr := prepareControlledProfileRead(file)
	if prepareErr != nil {
		_ = file.Close()
		return 0, cacheControlled, prepareErr
	}
	started := time.Now()
	read, readErr := io.CopyBuffer(writeOnlyDiscard{}, profileReaderOnly{file}, make([]byte, 1<<20))
	duration := time.Since(started)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return 0, cacheControlled, errors.Join(readErr, closeErr)
	}
	if read != controlledHashFileBytes {
		return 0, cacheControlled, io.ErrUnexpectedEOF
	}
	return duration, cacheControlled, nil
}

func measureFullHash(ctx context.Context, filename string) (time.Duration, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	file, err := os.Open(filename)
	if err != nil {
		return 0, false, err
	}
	cacheControlled, prepareErr := prepareControlledProfileRead(file)
	if prepareErr != nil {
		_ = file.Close()
		return 0, cacheControlled, prepareErr
	}
	started := time.Now()
	_, hashErr := hashutil.CalculateReaderHash(file, hashutil.AlgorithmBLAKE3)
	duration := time.Since(started)
	closeErr := file.Close()
	if hashErr != nil || closeErr != nil {
		return 0, cacheControlled, errors.Join(hashErr, closeErr)
	}
	return duration, cacheControlled, nil
}

func medianDuration(values []time.Duration) time.Duration {
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return values[len(values)/2]
}
