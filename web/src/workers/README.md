# Testing Web Workers

`hash.test.ts` is a browser contract test. It runs in headless Chrome as part of
the normal `vp test` / `make web-test` gate and covers both the small-file path
and the backend-compatible quick-hash path for files over 100 MiB.

The 20 x 50 MiB throughput case lives separately in `hash.perf.test.ts`. It is
never selected by the normal test gate; run it explicitly when changing hash
worker performance:

```bash
vp run test:hash-perf
```

Set `PLAYWRIGHT_CHANNEL` to override the default `chrome` channel in either
test mode.
