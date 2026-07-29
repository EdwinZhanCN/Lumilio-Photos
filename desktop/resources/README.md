# Bundled native runtime resources

The desktop app ships the native media tools the server needs because a
packaged app has no package-manager `PATH` to rely on. These binaries are large
and platform-specific, so they are staged here before the packaging scripts
assemble the app.

`desktop/scripts/fetch-resources.sh` downloads macOS ffmpeg, ffprobe, and
exiftool at pinned versions with SHA-256 verification. Its PowerShell
counterpart stages the Windows binaries.

Expected macOS layout:

```text
resources/
├── ffmpeg/
│   ├── ffmpeg
│   └── ffprobe
└── exiftool/
    ├── exiftool
    └── lib/
```

ExifTool is a Perl script rather than a self-contained binary. Its `lib/`
directory must remain beside the script so its modules can be resolved.

The supervisor resolves these tools relative to the packaged resources
directory. In local development, an absent bundled tool falls back to `PATH`.
SQLite and SQLite Vec1 are linked into the Go application; there is no
separate database runtime or vector extension to stage.

## Provenance

- `ffmpeg` / `ffprobe`: pinned native static builds, with the exact URL and
  SHA-256 recorded in the fetch script.
- `exiftool`: the pinned Image-ExifTool Perl distribution, including its
  adjacent module tree.
- `libvips`: collected from the build host with `dylibbundler` on macOS or the
  MinGW DLL-closure step on Windows.

## Local development

Run `make desktop-dev`. Staging resources is optional when the media tools and
libvips are already installed on the development machine.
