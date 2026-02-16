# Cloudflare Infrastructure

This directory contains Cloudflare deployment assets for the Studio plugin runtime.

## Components

- `registry-worker/`: public plugin registry API (D1 + KV + R2 bindings)
- `scripts/publish-plugin.mjs`: upload plugin artifacts and register releases

## Typical flow

1. Build plugin artifacts into a versioned folder.
2. Run `publish-plugin.mjs` to upload to R2 and upsert metadata in D1.
3. Studio frontend fetches catalog/manifest from the registry worker.
