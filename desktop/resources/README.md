# Bundled native runtime resources

The desktop app ships its own copies of the native tools the server needs,
because a packaged macOS app has no system `PATH` or package manager to rely on.
These binaries are **large and platform-specific**, so they are not committed to
git — they are staged here before `desktop/scripts/build-macos.sh` assembles the
`.app` bundle.

**Quick start:** `desktop/scripts/fetch-resources.sh` downloads ffmpeg, ffprobe,
and exiftool at pinned versions with SHA-256 verification (defaults to arm64).
PostgreSQL is the one piece it does not fetch — see below.

Expected layout (per architecture, `darwin-arm64` and/or `darwin-amd64`):

```
resources/
├── postgres/17/darwin-arm64/
│   ├── bin/        postgres, initdb, pg_ctl, pg_isready, createdb, pg_dump, pg_restore
│   ├── lib/        (incl. pgvector: vector.dylib under lib/postgresql)
│   └── share/postgresql/
├── ffmpeg/
│   ├── ffmpeg      native static build (arm64)
│   └── ffprobe     native static build (arm64)
└── exiftool/
    ├── exiftool    Perl script (EXIFTOOL_PATH points here)
    └── lib/        ExifTool's Perl modules — MUST sit next to the script
```

> exiftool is a **Perl script**, not a self-contained binary: it runs on macOS's
> system Perl (`/usr/bin/perl`, present but deprecated by Apple). The `lib/`
> directory must stay beside the `exiftool` script — the script locates its
> modules relative to itself. If Apple ever removes system Perl, a Perl
> interpreter would need to be bundled too.

The supervisor resolves these at runtime relative to the bundle's `Resources`
directory (`ResourcesDir()` in `supervisor/resources.go`). When a tool is absent
(e.g. local `make desktop-dev`), the server falls back to resolving it via
`PATH`, so development works against Homebrew-installed tools.

## Where the binaries come from

`fetch-resources.sh` pins the exact URLs + SHA-256 for ffmpeg/ffprobe/exiftool;
bump the pins (or override via env) when you update versions.

- **PostgreSQL 17 + pgvector + pg_textsearch + zhparser**: built from source — see
  `.github/workflows/build-postgres.yml`. Not fetched by `fetch-resources.sh`,
  because Homebrew/prebuilt PostgreSQL is not relocatable (absolute-path dylib
  links + baked-in paths break when moved into the bundle).
- **ffmpeg / ffprobe**: native static macOS builds. arm64 from
  [osxexperts.net](https://www.osxexperts.net) (`ffmpeg<ver>arm.zip` /
  `ffprobe<ver>arm.zip`); Intel from [evermeet.cx](https://evermeet.cx) (override
  the URLs + checksums for an amd64 bundle). Each zip's payload is a single
  self-contained binary (no dylibbundler needed). ~50MB each.
- **exiftool**: the `Image-ExifTool-<ver>.tar.gz` Perl distribution from
  [exiftool.org](https://exiftool.org) (the `exiftool` script + `lib/`), not the
  `.pkg` installer. ~7MB.
- **libvips + its dependency tree**: not staged here. `dylibbundler` collects it
  from the build host into `Contents/Frameworks/` during `build-macos.sh`
  (`brew install vips dylibbundler` first). libraw rides along as a libvips
  delegate.

## Local development

`make desktop-dev PG_BIN_DIR=/opt/homebrew/opt/postgresql@17/bin` points the
supervisor at a locally installed PostgreSQL via `LUMILIO_PG_BIN_DIR`, so you do
not need to stage anything here to run the app in development.
