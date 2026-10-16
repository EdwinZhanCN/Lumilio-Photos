// Package raw is the CGo LibRaw boundary for camera RAW files.
// [LibRawProcessor] renders RAW data to TIFF
// ([LibRawProcessor.RenderToTIFFPath]), extracts the embedded JPEG preview
// ([LibRawProcessor.ExtractEmbeddedWithLibRawPath]), and reads orientation.
// [Detector] and [IsRAWFile] recognise RAW formats.
//
// libvips cannot decode RAW in every build, so full renders always go through
// LibRaw to TIFF first.
//
//atlas:group media
package raw
