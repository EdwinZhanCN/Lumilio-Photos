# Cloudflare Plugin Registry Worker

Public read-only registry for Studio CDN plugins.

## Resources

- D1: `lumilio_plugin_registry`
- KV: `lumilio_plugin_cache`
- R2: `lumilio-plugin-artifacts`

## API

- `GET /v1/catalog?panel=plugins`
- `GET /v1/plugins/:pluginId/manifest`
- `GET /v1/plugins/:pluginId/manifest/:version`
- `GET /v1/revocations`

## Local setup

1. Create D1/KV/R2 resources.
2. Fill bindings in `wrangler.toml`.
3. Apply migrations:

```bash
wrangler d1 execute lumilio_plugin_registry --file ./migrations/0001_init.sql
wrangler d1 execute lumilio_plugin_registry --file ./migrations/0002_panel_plugins.sql
```

4. Run locally:

```bash
wrangler dev
```

## Production notes

- Configure CORS via `ALLOWED_ORIGIN`:
  - `*` for Docker or multi-domain frontend deployments.
  - Comma-separated exact origins for strict mode.
- Keep API responses short-lived cache (`max-age=60`) and store hot entries in KV.
- Keep R2 artifacts immutable with versioned object keys.
