# Cloudflare Infrastructure

This directory contains Cloudflare deployment assets for the Studio plugin runtime.

## Components

- `registry-worker/`: public plugin registry API (D1 + KV + R2 bindings)
- `scripts/publish-plugin.mjs`: upload plugin artifacts and register releases

## Typical flow

1. Build plugin artifacts into a versioned folder.
2. Run `publish-plugin.mjs` to upload to R2 and upsert metadata in D1.
3. Studio frontend fetches catalog/manifest from the registry worker.

## Publish script notes

- `publish-plugin.mjs` now enforces:
  - runtime manifest signature must be real (placeholder signatures are rejected)
  - runtime entry URLs must be real (placeholder `example.com` URLs are rejected)
- Use `--cdn-origin` to rewrite `entries.ui` and `entries.runner` for the release:
  - `--cdn-origin https://cdn.your-domain.com`
- Provide signing key via env vars:
  - `PLUGIN_SIGNING_PRIVATE_KEY_PEM`
  - optional `PLUGIN_SIGNING_KEY_ID`
- Script defaults:
  - uses `infra/cloudflare/registry-worker/wrangler.toml`
  - uses remote Cloudflare resources (`--remote`)
  - uses D1 binding `DB` when `--db` is omitted
- Uploaded artifacts are written with immutable cache headers:
  - `Cache-Control: public, max-age=31536000, immutable`

## R2 CORS for plugin runtime

Studio loads plugin UI modules via cross-origin dynamic import. The R2 bucket
behind `cdn.lumilio.org` must return CORS headers for browser module loading.

For Docker/multi-domain frontend deployments, apply the wildcard CORS profile:

```bash
wrangler r2 bucket cors set lumilio-plugin-artifacts \
  --file infra/cloudflare/r2-cors.public.json
```

Verify:

```bash
wrangler r2 bucket cors list lumilio-plugin-artifacts
curl -I \
  -H 'Origin: https://example.invalid' \
  https://cdn.lumilio.org/plugins/com.lumilio.border/0.2.0/ui.mjs
```

Expected response header includes:
- `Access-Control-Allow-Origin: *`
