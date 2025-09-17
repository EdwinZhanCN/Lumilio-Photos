
### Support Image Types
- JPEG
- PNG
- WEBP

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

### Command Line Tools
(Required For Video/Image Processing)
- exiftool
- dcraw/libraw/ImageMagick(Not Recommand)
- ffmpeg
