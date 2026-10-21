// Package classify holds the pure, dependency-free math for zero-shot
// classification: prompt-ensemble prototypes, contrastive scoring against
// L2-normalized embeddings, and confidence calibration. Keeping it free of DB,
// ML, and imaging (libvips) dependencies makes it cheap to unit-test and safe to
// import from both the service and queue layers.
//
//atlas:group domain
package classify
