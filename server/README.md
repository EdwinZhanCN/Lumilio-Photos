
### Support Image Types
- JPEG
- PNG
- WEBP

### Supported Video Types
- MP4
- MOV
- AVI
- MKV
- WEBM
- FLV
- WMV
- M4V

### Supported Audio Types
- MP3
- AAC
- M4A
- FLAC
- WAV
- OGG
- AIFF
- WMA

### Supported RAW Types
- CR2 (Canon)
- CR3 (Canon)
- NEF (Nikon)
- ARW (Sony)
- DNG (Adobe Digital Negative)
- ORF (Olympus)
- RW2 (Panasonic)
- PEF (Pentax)
- RAF (Fujifilm)
- MRW (Minolta/Konica Minolta)
- SRW (Samsung)
- RWL (Leica)
- X3F (Sigma)

Notes:
- Detection is performed primarily by file extension; for several formats additional magic-byte checks are used when available.
- The detector also attempts to locate and extract embedded JPEG previews from RAW files where present.
- Video and audio files are automatically transcoded to web-compatible formats (MP4/H.264 for video, MP3 for audio) for optimal playback performance.
- Original files are preserved alongside web-optimized versions when beneficial (e.g., 4K videos keep both original and 1080p versions).

### Command Line Tools
(Required For Video/Image/Audio Processing)

#### Essential Tools
- **exiftool** - Metadata extraction for all asset types (photos, videos, audio)
- **ffmpeg** - Video transcoding, audio transcoding, thumbnail generation, and media analysis
- **ffprobe** - Media file analysis (part of ffmpeg package)

#### Image Processing
- **dcraw/libraw** - RAW image processing (ImageMagick not recommended)

#### Installation Instructions

##### macOS (using Homebrew)
```bash
brew install exiftool ffmpeg dcraw
```

##### Ubuntu/Debian
```bash
sudo apt update
sudo apt install exiftool ffmpeg dcraw
```

##### CentOS/RHEL/Fedora
```bash
# Fedora
sudo dnf install perl-Image-ExifTool ffmpeg dcraw

# CentOS/RHEL (requires EPEL)
sudo yum install epel-release
sudo yum install perl-Image-ExifTool ffmpeg dcraw
```

##### Windows
1. **exiftool**: Download from https://exiftool.org/
2. **ffmpeg**: Download from https://ffmpeg.org/download.html
3. **dcraw**: Download from https://www.dechifro.org/dcraw/

#### Verification
Verify installation with:
```bash
exiftool -ver
ffmpeg -version
ffprobe -version
dcraw
```

#### Processing Capabilities
- **Videos**: Transcoded to web-compatible MP4 (H.264) with smart resolution handling
- **Audio**: Transcoded to web-compatible MP3 with quality optimization
- **Photos**: EXIF extraction, thumbnail generation, and RAW processing
- **Thumbnails**: Generated for all media types including video frames and audio waveforms
