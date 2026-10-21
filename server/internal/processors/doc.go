// Package processors runs the read/compute stages of asset processing: ingest
// of a staged source, metadata extraction (exiftool and ffprobe), thumbnails
// and video frames, transcodes, and ML inputs.
//
// [AssetProcessor] receives [AssetStageArgs] and returns immutable results
// ([DerivativeResult], [DerivedArtifact], [MetadataResult]). Processors
// publish derived files only through
// [AssetProcessor.PublishThumbnailTask] and never write
// catalog rows themselves: background results are acknowledged by the commit
// coordinator.
//
// Metadata projection: exiftool output is kept verbatim in assets.exif_raw and
// projected into two models. Common searchable facts (capture time and offset,
// GPS, dimensions, duration, rating) are typed asset columns; embedded
// keywords become asset_tags; specific_metadata holds only media-type-specific
// display and filter facts and never duplicates the common columns. A metadata
// retry replaces specific_metadata with the current schema but preserves an
// existing description and never overwrites rating or keyword relations after
// the first successful extraction.
//
// A metadata subprocess succeeds only with valid output, a clean exit, and
// intact input; an expected stdin EPIPE must not mask valid output, and every
// started process is waited for. Decision:
// .agents/decisions/2026-09-18-processing-convergence.md.
//
//atlas:group ingest
package processors
