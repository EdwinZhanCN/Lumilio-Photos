// Package upload implements the HTTP upload wire protocol for batch and
// chunked transfers.
//
// Multipart field names encode the session and chunk position
// ([GenerateSingleFieldName], [GenerateChunkFieldName], [ParseFileField]).
// [SessionManager] tracks in-flight chunked sessions per user and repository,
// and [ChunkMerger] assembles received chunks into a repository staging file
// before the sourcing pipeline takes over.
//
//atlas:group ingest
package upload
