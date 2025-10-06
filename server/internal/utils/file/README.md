# File Validator

A comprehensive file validation system for the Lumilio Photos backend that ensures consistency across all file upload and processing operations.

## Overview

The file validator provides centralized validation logic for all supported file types, including:
- **Photos**: Standard formats (JPEG, PNG, WEBP, etc.) and RAW camera formats
- **Videos**: All common video formats (MP4, MOV, AVI, MKV, etc.)
- **Audio**: All common audio formats (MP3, FLAC, WAV, etc.)

## Supported Formats

### Photos (Standard)
- **JPEG/JPG** - Joint Photographic Experts Group
- **PNG** - Portable Network Graphics
- **WEBP** - Google's WebP format
- **GIF** - Graphics Interchange Format
- **BMP** - Bitmap Image File
- **TIFF/TIF** - Tagged Image File Format
- **HEIC/HEIF** - High Efficiency Image Format (Apple)

### Photos (RAW Camera Formats)
- **CR2** - Canon RAW 2
- **CR3** - Canon RAW 3
- **NEF** - Nikon Electronic Format
- **ARW** - Sony Alpha RAW
- **DNG** - Adobe Digital Negative (universal RAW)
- **ORF** - Olympus RAW Format
- **RW2** - Panasonic RAW 2
- **PEF** - Pentax Electronic Format
- **RAF** - Fujifilm RAW Format
- **MRW** - Minolta RAW
- **SRW** - Samsung RAW
- **RWL** - Leica RAW
- **X3F** - Sigma RAW Format

### Videos
- **MP4** - MPEG-4 Part 14
- **MOV** - Apple QuickTime Movie
- **AVI** - Audio Video Interleave
- **MKV** - Matroska Video
- **WEBM** - WebM Video
- **FLV** - Flash Video
- **WMV** - Windows Media Video
- **M4V** - Apple iTunes Video
- **3GP** - 3GPP Multimedia
- **MPG/MPEG** - MPEG Video
- **M2TS/MTS** - MPEG Transport Stream
- **OGV** - Ogg Video

### Audio
- **MP3** - MPEG Audio Layer III
- **AAC** - Advanced Audio Coding
- **M4A** - MPEG-4 Audio
- **FLAC** - Free Lossless Audio Codec
- **WAV** - Waveform Audio File
- **OGG** - Ogg Vorbis
- **AIFF** - Audio Interchange File Format
- **WMA** - Windows Media Audio
- **OPUS** - Opus Interactive Audio Codec
- **OGA** - Ogg Audio
