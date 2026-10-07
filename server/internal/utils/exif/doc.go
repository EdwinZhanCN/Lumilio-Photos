// Package exif extracts photo, video, and audio metadata by streaming file
// bytes through an external exiftool process.
//
// An [Extractor] is configured once from [Config] and serves
// [Extractor.ExtractFromStream] and [Extractor.ExtractBatch]; each request is
// bounded by the configured timeout and returns a [MetadataResult]. The tool
// path comes from the strict runtime manifest, never from a search. Console
// windows are suppressed on Windows through
// [server/internal/utils/sysproc.HideConsole].
//
//atlas:group media
package exif
