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
//atlas:group ingest
package processors
