// Package search answers library queries by fusing independent retrievers.
//
// Each [Retriever] is self-thresholded and returns [Candidate] values:
// [EmbeddingRetriever] (Vec1 semantic vectors with a cosine floor),
// [TextRetriever] for places, and [BleveOCRRetriever] over the OCR sidecar.
// [FuseSet] merges channels with weighted reciprocal-rank fusion and no TopK;
// [AggregateService] runs the retrievers for a [Request], applies the
// [Filter], and [HydrateAssets] loads the ranked catalog rows.
//
//atlas:group domain
package search
