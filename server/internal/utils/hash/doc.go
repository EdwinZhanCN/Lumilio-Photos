// Package hash computes content identity.
//
// Exact identity is full-file BLAKE3 plus size: [CalculateLayeredBLAKE3] and
// [CalculateLayeredBLAKE3Reader] return a [LayeredHashResult] whose
// ContentHash is authoritative. For large files the result also carries a
// versioned quick fingerprint (size plus fixed first/last chunks) that is only
// a duplicate precheck hint, never identity. The browser upload fingerprint
// mirrors the same policy.
//
//atlas:group media
package hash
