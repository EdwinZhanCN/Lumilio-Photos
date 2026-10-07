// Package file classifies media files: it maps filenames and MIME types to a
// catalog Asset type ([DetermineAssetType], [DetermineAssetTypeWithFilename]),
// owns the supported-extension registry ([IsSupported],
// [GetSupportedExtensions]), and validates uploads ([ValidateFile]).
//
// It is the single answer to "is this a photo, video, or audio file Lumilio
// manages?" Scanners, upload, and processors all ask here rather than keeping
// their own extension lists.
//
//atlas:group media
package file
