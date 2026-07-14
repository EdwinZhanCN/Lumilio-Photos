# Lumilio Docs Site

This directory contains the public Lumilio Photos documentation site. It is a VitePress site managed with Vite+ package commands.

## Local Commands

```bash
vp install
vp run dev
vp run build
vp run preview
```

Refresh the generated API reference from the root workspace:

```bash
make dto
```

The VitePress source root is `docs/`. Production output is generated at:

```text
docs/.vitepress/dist
```

## R2 media

Product images and demo videos are served from the `lumilio-docs-media` R2
bucket through `https://media.docs.lumilio.org`. Their repository-relative
paths are mapped to immutable SHA-256 object keys in
`docs/.vitepress/media-manifest.json`; VitePress rewrites matching `/images/*`
and `/videos/*` URLs during dev and build.

The source directory for new or replacement media is local-only and ignored:

```text
site/media/images/
site/media/videos/
```

For new or replacement media, place the files under `site/media` and run these
commands from `site/` on a machine authenticated with Wrangler:

```bash
pnpm media:manifest
pnpm media:sync
pnpm media:verify
```

`media:sync` explicitly uses Wrangler's `--remote` mode, refuses a file whose
SHA-256 differs from the manifest, and uploads with immutable caching metadata.
Do not delete a local media file until
`media:verify` succeeds and the Pages preview renders it from the media domain.
