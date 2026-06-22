<#
.SYNOPSIS
    Build the Lumilio Photos Windows app bundle and (optionally) an NSIS installer.

.DESCRIPTION
    Windows counterpart to scripts/build-macos.sh. Like the macOS build, this does
    NOT use `wails3 build`: the UI is served by the in-process Go API server (a
    WebAuthn/passkey requirement), so the bundle is just the Go binary plus the
    bundled native runtime (PostgreSQL, ffmpeg, exiftool) and the libvips DLLs.

    Layout produced under build\windows\app\ (and installed by installer.nsi):
        lumilio-photos.exe
        *.dll                              libvips + dependency closure
        resources\postgres\17\windows-amd64\{bin,lib,share}
        resources\ffmpeg\{ffmpeg,ffprobe}.exe
        resources\exiftool\exiftool.exe (+ exiftool_files\)
        resources\web\                     (vp build output)
        resources\lib\vips-modules-*\      (libvips dynamic modules)

    Prerequisites (staged before running):
        desktop/scripts/fetch-resources-windows.ps1   ffmpeg/exiftool/libvips SDK
        desktop/resources/postgres/17/windows-amd64    PostgreSQL 17 + pgvector
        a MinGW-w64 gcc on PATH (CGO compiler) and `vp` (Vite+) for the web build.

.PARAMETER MakeInstaller
    Also build the NSIS installer (requires makensis on PATH).
#>
param(
    [switch]$MakeInstaller,
    [string]$Version = $env:LUMILIO_VERSION
)

$ErrorActionPreference = "Stop"
if (-not $Version) { $Version = "0.0.0" }

$ScriptDir  = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopDir = Split-Path -Parent $ScriptDir
$Root       = Split-Path -Parent $DesktopDir
$Resources  = Join-Path $DesktopDir "resources"
$VipsDir    = Join-Path $DesktopDir "build\windeps\vips"
$BuildDir   = Join-Path $DesktopDir "build\windows"
$AppDir     = Join-Path $BuildDir "app"
$ResDest    = Join-Path $AppDir "resources"

if (-not (Test-Path (Join-Path $VipsDir "bin\libvips-42.dll"))) {
    throw "libvips SDK not found at $VipsDir. Run scripts/fetch-resources-windows.ps1 first."
}
$pgBin = Join-Path $Resources "postgres\17\windows-amd64\bin"
if (-not (Test-Path (Join-Path $pgBin "postgres.exe"))) {
    Write-Warning "PostgreSQL not staged at $pgBin — the installed app will fail to start until it is bundled."
}

Write-Host "==> Cleaning previous bundle"
if (Test-Path $AppDir) { Remove-Item -Recurse -Force $AppDir }
New-Item -ItemType Directory -Force -Path $AppDir, $ResDest | Out-Null

$WebDist = Join-Path $Root "web\dist"
if (Test-Path (Join-Path $WebDist "index.html")) {
    Write-Host "==> Reusing prebuilt web frontend at $WebDist"
} else {
    Write-Host "==> Building web frontend (vp build)"
    if (-not (Get-Command vp -ErrorAction SilentlyContinue)) {
        throw "vp (Vite+) not found and no prebuilt web\dist present; build the web bundle first."
    }
    Push-Location (Join-Path $Root "web")
    try { vp build } finally { Pop-Location }
    if (-not (Test-Path (Join-Path $WebDist "index.html"))) { throw "web build missing at $WebDist" }
}
Copy-Item (Join-Path $WebDist "*") (Join-Path $ResDest "web") -Recurse -Force

Write-Host "==> Building Go binary (CGO + libvips, $Version)"
# Point cgo at the libvips SDK headers/import libs and make pkg-config (used by
# govips) resolve from the SDK.
$env:CGO_ENABLED       = "1"
$env:GOOS              = "windows"
$env:GOARCH            = "amd64"
$env:PKG_CONFIG_PATH   = (Join-Path $VipsDir "lib\pkgconfig")
$env:CGO_CFLAGS        = "-I" + (Join-Path $VipsDir "include")
$env:CGO_LDFLAGS       = "-L" + (Join-Path $VipsDir "lib")
# Statically link the MinGW C/C++ runtime so the .exe does not need libgcc/
# libstdc++/libwinpthread DLLs alongside it.
$ldflags = "-s -w -H windowsgui -extldflags `"-static-libgcc -static-libstdc++`""
$exe = Join-Path $AppDir "lumilio-photos.exe"
Push-Location $DesktopDir
try {
    go build -ldflags "$ldflags" -o "$exe" .
} finally { Pop-Location }
if (-not (Test-Path $exe)) { throw "go build did not produce $exe" }

Write-Host "==> Bundling libvips runtime DLLs"
# The libvips "web" SDK's bin\ is a self-contained dependency closure; copying
# every DLL next to the .exe satisfies the Windows loader without manual walking.
Copy-Item (Join-Path $VipsDir "bin\*.dll") $AppDir -Force

Write-Host "==> Staging libvips dynamic modules"
$modDir = Get-ChildItem -Path (Join-Path $VipsDir "lib") -Directory -Filter "vips-modules-*" -ErrorAction SilentlyContinue | Select-Object -First 1
if ($modDir) {
    $dest = Join-Path $ResDest ("lib\" + $modDir.Name)
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Copy-Item (Join-Path $modDir.FullName "*.dll") $dest -Force
} else {
    Write-Warning "no vips-modules-* directory in the libvips SDK; HEIC/AVIF module loaders may be unavailable"
}

Write-Host "==> Staging bundled runtime resources"
foreach ($name in @("ffmpeg", "exiftool", "postgres")) {
    $src = Join-Path $Resources $name
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $ResDest $name) -Recurse -Force
    } else {
        Write-Warning "missing $src — bundle will fall back to PATH at runtime"
    }
}

Write-Host "==> Built: $AppDir"

if ($MakeInstaller) {
    Write-Host "==> Building NSIS installer"
    if (-not (Get-Command makensis -ErrorAction SilentlyContinue)) {
        throw "makensis not found; install NSIS (choco install nsis) to build the installer."
    }
    $nsi = Join-Path $DesktopDir "packaging\windows\installer.nsi"
    makensis "/DAPP_DIR=$AppDir" "/DOUT_DIR=$BuildDir" "/DAPP_VERSION=$Version" "$nsi"
    $installer = Join-Path $BuildDir "Lumilio-Photos-Setup-$Version.exe"
    if (Test-Path $installer) { Write-Host "==> Built installer: $installer" }
}
