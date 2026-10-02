// Package phash computes perceptual hashes for near-duplicate detection.
// [ComputeFromReader] hashes a decoded image, [ToVector] and [FromVector]
// convert hashes to and from the catalog vector column, and [Cluster] groups
// hashes whose [HammingDistance] is within a threshold.
//
//atlas:group media
package phash
