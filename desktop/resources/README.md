# Bundled native runtime resources

The desktop app ships its own copies of the native tools the server needs,
because a packaged macOS app has no system `PATH` or package manager to rely on.
These binaries are **large and platform-specific**, so they are not committed to
git — they are staged here by CI (or manually) before `desktop/scripts/build-macos.sh`
assembles the `.app` bundle.

Expected layout (per architecture, `darwin-arm64` and/or `darwin-amd64`):

```
resources/
├── postgres/16/darwin-arm64/
│   ├── bin/        postgres, initdb, pg_ctl, pg_isready, createdb, pg_dump, pg_restore
│   ├── lib/        (incl. pgvector: vector.dylib under lib/postgresql)
│   └── share/postgresql/
├── ffmpeg/
│   ├── ffmpeg      static build (BtbN / evermeet.cx)
│   └── ffprobe
└── exiftool/
    └── exiftool    official macOS standalone (bundles its own Perl)
```

The supervisor resolves these at runtime relative to the bundle's `Resources`
directory (`ResourcesDir()` in `supervisor/resources.go`). When a tool is absent
(e.g. local `make desktop-dev`), the server falls back to resolving it via
`PATH`, so development works against Homebrew-installed tools.

## Where the binaries come from

- **PostgreSQL 16 + pgvector**: built from source in CI — see
  `.github/workflows/build-postgres.yml`.
- **ffmpeg / ffprobe**: a static macOS build (~70-80MB) with VideoToolbox.
- **exiftool**: the official macOS standalone distribution (~6MB).
- **libvips + its dependency tree**: not staged here. `dylibbundler` collects it
  from the build host into `Contents/Frameworks/` during `build-macos.sh`
  (`brew install vips dylibbundler` first). libraw rides along as a libvips
  delegate.

## Local development

`make desktop-dev PG_BIN_DIR=/opt/homebrew/opt/postgresql@16/bin` points the
supervisor at a locally installed PostgreSQL via `LUMILIO_PG_BIN_DIR`, so you do
not need to stage anything here to run the app in development.
