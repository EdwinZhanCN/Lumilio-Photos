// Package vectorindex owns the rebuildable Vec1 semantic index. Authoritative
// embeddings stay in ordinary catalog tables; the index is a derived query
// structure. Below 5,000 semantic rows it is an exact flat scan ([ModeFlat]);
// larger libraries train a PQ ANN model ([ModeANN]) whose candidates are
// filtered inside Vec1 and exactly re-ranked from the authoritative vectors.
// [Reconcile] restores the index at startup and [Maintain] keeps it current.
//
//atlas:group catalog
package vectorindex
