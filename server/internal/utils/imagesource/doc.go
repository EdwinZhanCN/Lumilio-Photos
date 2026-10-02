// Package imagesource turns a stored photo into the decodable image every
// consumer needs. [OpenPhoto] resolves RAW files to their embedded preview and
// falls back to a full LibRaw render; [ProcessMLImage] and its reader/bytes
// variants prepare fixed-size RGB tensors for a [Purpose] (semantic,
// BioCLIP, OCR, face); [PrepareSemanticThumbnail] produces an in-memory WebP for Lumen
// without writing to the repository.
//
//atlas:group media
package imagesource
