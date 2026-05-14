package imaging

import (
	"log/slog"
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
)

// libvips global state is initialized once per process. ConcurrencyLevel is set
// to 1 so each vips op runs single-threaded internally; outer parallelism comes
// from River worker goroutines (queue_setup.go). That combination:
//   - avoids libvips/libexif races that produced corrupted ("rainbow stripe") WebP
//     when separate AutoRotate + Process ops ran concurrently under bimg;
//   - in Docker import benchmarks (~750-thumb set), beat libvips default
//     concurrency: ~375 s active with CL=1 vs ~402 s default; lower peak RAM.
//     Default internal threading oversubscribed this workload.
var (
	vipsStartOnce sync.Once
	vipsStarted   bool
)

// StartVips initializes the libvips runtime exactly once. Safe to call multiple
// times; subsequent calls are no-ops.
func StartVips() {
	vipsStartOnce.Do(func() {
		vips.LoggingSettings(func(domain string, level vips.LogLevel, msg string) {
			switch level {
			case vips.LogLevelError, vips.LogLevelCritical:
				slog.Error("libvips", "domain", domain, "msg", msg)
			case vips.LogLevelWarning:
				slog.Warn("libvips", "domain", domain, "msg", msg)
			default:
				slog.Debug("libvips", "domain", domain, "msg", msg)
			}
		}, vips.LogLevelWarning)

		// MaxCache*: 0 — thumbnails use fresh buffers each time; operation cache
		// never hits and only adds global lock contention.
		vips.Startup(&vips.Config{
			ConcurrencyLevel: 1,
			MaxCacheFiles:    0,
			MaxCacheMem:      0,
			MaxCacheSize:     0,
			CollectStats:     false,
		})
		vipsStarted = true
	})
}

// ShutdownVips releases libvips global state. Call once at process exit (deferred
// from main). After Shutdown, no further imaging.* calls should occur.
func ShutdownVips() {
	if !vipsStarted {
		return
	}
	vips.Shutdown()
	vipsStarted = false
}
