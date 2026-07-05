# Fetches the Windows media-tool binaries (ffmpeg/ffprobe/exiftool) into
# desktop/resources/ with pinned versions and SHA-256 verification — the
# Windows counterpart of fetch-resources.sh. PostgreSQL is staged separately
# (see .github/workflows/build-postgres.yml, artifact postgres-windows-amd64).
#
# Override any pin via env: FFMPEG_URL/FFMPEG_SHA256, EXIFTOOL_URL/EXIFTOOL_SHA256.

$ErrorActionPreference = "Stop"

$FfmpegUrl = if ($env:FFMPEG_URL) { $env:FFMPEG_URL } else {
    "https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-8.1.2-essentials_build.zip"
}
$FfmpegSha = if ($env:FFMPEG_SHA256) { $env:FFMPEG_SHA256 } else {
    "db580001caa24ac104c8cb856cd113a87b0a443f7bdf47d8c12b1d740584a2ec"
}

# exiftool.org no longer serves files directly; downloads live on SourceForge.
# Use the automation endpoint (downloads.sourceforge.net redirects straight to
# a mirror file) — the /files/.../download page can serve an HTML interstitial
# to non-browser clients, which breaks the checksum. SHA pin still verifies.
$ExiftoolUrl = if ($env:EXIFTOOL_URL) { $env:EXIFTOOL_URL } else {
    "https://downloads.sourceforge.net/project/exiftool/exiftool-13.59_64.zip"
}
$ExiftoolSha = if ($env:EXIFTOOL_SHA256) { $env:EXIFTOOL_SHA256 } else {
    "44b512b25af500724ba579d0a53c8fc5851628b692dd5e5d94ae4a15c2cba9ec"
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Resources = Join-Path (Split-Path -Parent $ScriptDir) "resources"
$Work = Join-Path $Resources ".downloads"
New-Item -ItemType Directory -Force -Path $Work | Out-Null

function Get-Verified([string]$Url, [string]$Sha, [string]$OutFile) {
    Write-Host "==> Downloading $Url"
    Invoke-WebRequest -Uri $Url -OutFile $OutFile
    $actual = (Get-FileHash -Algorithm SHA256 $OutFile).Hash.ToLowerInvariant()
    if ($actual -ne $Sha.ToLowerInvariant()) {
        throw "checksum mismatch for ${Url}: expected $Sha, got $actual (version bumped upstream? update the pin or override via env)"
    }
}

# --- ffmpeg + ffprobe (gyan.dev essentials build) ---
$ffzip = Join-Path $Work "ffmpeg.zip"
Get-Verified $FfmpegUrl $FfmpegSha $ffzip
$ffdir = Join-Path $Work "ffmpeg"
Remove-Item -Recurse -Force $ffdir -ErrorAction SilentlyContinue
Expand-Archive $ffzip -DestinationPath $ffdir
$ffbin = Get-ChildItem -Recurse -Path $ffdir -Filter "ffmpeg.exe" | Select-Object -First 1
if (-not $ffbin) { throw "ffmpeg.exe not found in archive" }
$dest = Join-Path $Resources "ffmpeg"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item $ffbin.FullName (Join-Path $dest "ffmpeg.exe") -Force
Copy-Item (Join-Path $ffbin.DirectoryName "ffprobe.exe") (Join-Path $dest "ffprobe.exe") -Force
Write-Host "==> Staged $dest\ffmpeg.exe + ffprobe.exe"

# --- exiftool (windows zip: exiftool(-k).exe + exiftool_files) ---
$etzip = Join-Path $Work "exiftool.zip"
Get-Verified $ExiftoolUrl $ExiftoolSha $etzip
$etdir = Join-Path $Work "exiftool"
Remove-Item -Recurse -Force $etdir -ErrorAction SilentlyContinue
Expand-Archive $etzip -DestinationPath $etdir
$etexe = Get-ChildItem -Recurse -Path $etdir -Filter "exiftool(-k).exe" | Select-Object -First 1
if (-not $etexe) { throw "exiftool(-k).exe not found in archive" }
$dest = Join-Path $Resources "exiftool"
Remove-Item -Recurse -Force $dest -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $dest | Out-Null
# Renaming away the "-k" suffix removes the interactive pause-on-exit behavior.
Copy-Item $etexe.FullName (Join-Path $dest "exiftool.exe") -Force
$etfiles = Join-Path $etexe.DirectoryName "exiftool_files"
if (Test-Path $etfiles) {
    Copy-Item -Recurse $etfiles (Join-Path $dest "exiftool_files")
}
Write-Host "==> Staged $dest\exiftool.exe"

Remove-Item -Recurse -Force $Work
Write-Host "==> Done"
