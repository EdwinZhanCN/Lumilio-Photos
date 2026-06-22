<#
.SYNOPSIS
    Download the bundled Windows media tools (ffmpeg, ffprobe, exiftool) and the
    libvips SDK into desktop/resources/ with pinned versions.

.DESCRIPTION
    Windows counterpart to scripts/fetch-resources.sh. PostgreSQL is NOT fetched
    here — a relocatable PostgreSQL 17 + pgvector bundle is staged separately
    (see .github/workflows/release-desktop.yml). The libvips SDK is downloaded to
    a build directory because it provides both the headers/import libs used to
    cgo-compile the Go binary and the runtime DLLs bundled next to the .exe.

    Re-running is safe: already-present files are reused.

.PARAMETER VipsVersion
    libvips Windows build version (libvips/build-win64-mxe release tag, no 'v').

.PARAMETER FfmpegVersion
    GyanD/codexffmpeg release version.

.PARAMETER ExifToolVersion
    ExifTool version (matches scripts/fetch-resources.sh default).
#>
param(
    [string]$VipsVersion   = $env:VIPS_VERSION,
    [string]$FfmpegVersion = $env:FFMPEG_VERSION,
    [string]$ExifToolVersion = $env:EXIFTOOL_VERSION
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

if (-not $VipsVersion)     { $VipsVersion = "8.16.0" }
if (-not $FfmpegVersion)   { $FfmpegVersion = "7.1" }
if (-not $ExifToolVersion) { $ExifToolVersion = "13.59" }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopDir = Split-Path -Parent $ScriptDir
$Resources = Join-Path $DesktopDir "resources"
$BuildDeps = Join-Path $DesktopDir "build\windeps"
$Tmp = Join-Path $env:TEMP ("lumilio-winres-" + [System.Guid]::NewGuid().ToString("N"))

New-Item -ItemType Directory -Force -Path $Resources, $BuildDeps, $Tmp | Out-Null

function Get-File($url, $dest) {
    Write-Host "  downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $dest
}

# --- libvips SDK (headers + import libs + runtime DLLs) -----------------------
# The "web" variant covers the loaders we need (incl. HEIC/AVIF) and is fully
# self-contained: bin\*.dll is the complete runtime dependency closure.
$VipsDir = Join-Path $BuildDeps "vips"
if (-not (Test-Path (Join-Path $VipsDir "bin\libvips-42.dll"))) {
    $zip = Join-Path $Tmp "vips.zip"
    Get-File "https://github.com/libvips/build-win64-mxe/releases/download/v$VipsVersion/vips-dev-w64-web-$VipsVersion.zip" $zip
    $extract = Join-Path $Tmp "vips"
    Expand-Archive -Path $zip -DestinationPath $extract -Force
    # The archive contains a top-level vips-dev-w64-web-<ver>\ directory.
    $inner = Get-ChildItem -Path $extract -Directory | Select-Object -First 1
    if (Test-Path $VipsDir) { Remove-Item -Recurse -Force $VipsDir }
    Move-Item -Path $inner.FullName -Destination $VipsDir
    Write-Host "  libvips $VipsVersion staged at $VipsDir"
} else {
    Write-Host "  libvips: already present — skipping"
}

# --- ffmpeg / ffprobe --------------------------------------------------------
$FfmpegDir = Join-Path $Resources "ffmpeg"
New-Item -ItemType Directory -Force -Path $FfmpegDir | Out-Null
if (-not (Test-Path (Join-Path $FfmpegDir "ffmpeg.exe"))) {
    $zip = Join-Path $Tmp "ffmpeg.zip"
    Get-File "https://github.com/GyanD/codexffmpeg/releases/download/$FfmpegVersion/ffmpeg-$FfmpegVersion-essentials_build.zip" $zip
    $extract = Join-Path $Tmp "ffmpeg"
    Expand-Archive -Path $zip -DestinationPath $extract -Force
    $bin = Get-ChildItem -Path $extract -Recurse -Filter "ffmpeg.exe" | Select-Object -First 1
    Copy-Item (Join-Path $bin.DirectoryName "ffmpeg.exe")  (Join-Path $FfmpegDir "ffmpeg.exe")  -Force
    Copy-Item (Join-Path $bin.DirectoryName "ffprobe.exe") (Join-Path $FfmpegDir "ffprobe.exe") -Force
    Write-Host "  ffmpeg $FfmpegVersion staged"
} else {
    Write-Host "  ffmpeg: already present — skipping"
}

# --- exiftool ----------------------------------------------------------------
$ExifDir = Join-Path $Resources "exiftool"
New-Item -ItemType Directory -Force -Path $ExifDir | Out-Null
if (-not (Test-Path (Join-Path $ExifDir "exiftool.exe"))) {
    $zip = Join-Path $Tmp "exiftool.zip"
    Get-File "https://exiftool.org/exiftool-${ExifToolVersion}_64.zip" $zip
    $extract = Join-Path $Tmp "exiftool"
    Expand-Archive -Path $zip -DestinationPath $extract -Force
    # The Windows package ships "exiftool(-k).exe" plus an exiftool_files\ dir.
    $exe = Get-ChildItem -Path $extract -Recurse -Filter "exiftool(-k).exe" | Select-Object -First 1
    Copy-Item $exe.FullName (Join-Path $ExifDir "exiftool.exe") -Force
    $files = Join-Path $exe.DirectoryName "exiftool_files"
    if (Test-Path $files) {
        Copy-Item $files (Join-Path $ExifDir "exiftool_files") -Recurse -Force
    }
    Write-Host "  exiftool $ExifToolVersion staged"
} else {
    Write-Host "  exiftool: already present — skipping"
}

Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
Write-Host "==> Windows resources ready in $Resources (libvips SDK in $VipsDir)"
