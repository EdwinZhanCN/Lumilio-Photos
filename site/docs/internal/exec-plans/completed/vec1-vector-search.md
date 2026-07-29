# Vec1 Vector Search

Status: completed on 2026-07-28

## Goal

Replace `github.com/asg017/sqlite-vec-go-bindings` completely with the official
SQLite Vec1 0.7 extension while preserving the existing semantic-search,
classifier, face-clustering, authorization, and video-frame contracts.

## Final Contracts

- `search_embeddings.vector` and `face_items.embedding` are authoritative
  little-endian float32 BLOBs. Vec1 is a rebuildable semantic query structure.
- Schema generation 4 starts directly at `000003_vec1_baseline.up.sql` and
  stamps `PRAGMA user_version = 4`. Generation-3 catalogs are rejected with a
  delete-and-recreate instruction; the old migration bytes remain unchanged
  and are never executed for a new generation-4 catalog.
- Vec1 0.7 is vendored as its official public-domain single-file source. The
  recorded SHA-256 is
  `8571bb4f77f9547d11ad11e2f72e0de7d3b2ab44e7930151998bce9377ed4b86`.
  It is statically registered with scalar/AVX2 runtime dispatch on x86 and NEON
  on arm64.
- Semantic search is exact-flat below 5,000 rows. At 5,000 rows it trains a
  48-byte residual PQ model, samples at most 100,000 uniformly spaced rows, and
  retrains after a twofold population change. Training or install failure
  rebuilds the exact-flat index.
- Embedding space, owner, deletion state, and asset type are filtered inside
  Vec1 before candidate accumulation and verified against authoritative rows.
  ANN candidates use `nprobe=0.15`, an eightfold video-frame expansion, and are
  exact-reranked with `vec1_l2_distance` before business thresholds are applied.
- Strict semantic membership scans authoritative BLOBs exactly. Normal/loose
  ANN results do not claim completeness; callers may request the strict path.
- Face DBSCAN and classifier preview remain exact and use
  `vec1_cos_distance`. No face ANN table is created.
- Startup verifies authoritative/derived row parity and repairs Vec1 metadata
  and rows before serving. Model-swap reset returns the index to flat before
  refill.
- Backup manifest format 2 records the portable Vec1 release (`0.7`) rather
  than CPU-specific `vec1_info()` build features.

## Useful Decisions

- Historical migrations were not edited. The migration runner selects the
  active generation baseline, and sqlc parses that baseline only.
- Vec1 squared-L2 values stay squared in SQL for sorting and cutoff comparison;
  the Go boundary converts returned scores back to the existing L2 units.
- The old 4,096-vector sqlite-vec limit was removed. Vec1 requests have an
  explicit 65,536-vector work bound, after which set retrieval uses the exact
  authoritative path.
- Runtime TOML gained no vector tuning fields. The index policy is an
  implementation contract versioned with the schema and code.

## Validation Boundaries

- `sqlc generate`
- `make server-test`
- `make desktop-test`
- `make web-test`
- `cd site && vp run build`
- Real 768D semantic insert/filter/delete/query and exact-reranking integration
  tests.
- Real Vec1 PQ training/install/query plus ANN recall comparison against an
  exhaustive scalar-distance reference.
- Local native validation ran on macOS arm64. Linux and Windows CGo package
  prerequisites are explicit in CI; those native jobs remain the platform
  release gates.
