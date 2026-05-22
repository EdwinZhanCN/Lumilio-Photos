# Queue Worker Dependency DAG

This document describes the asynchronous worker graph in `server/internal/queue`.

Legend:
- Solid arrows are hard runtime dependencies: one worker enqueues the next worker as part of its normal flow.
- Dashed arrows are retry/dispatcher edges: the worker can re-enqueue another worker based on task selection.
- Some edges are conditional on asset type or metadata being present.

```mermaid
flowchart LR
  %% Roots
  SR[ScanRepositoryWorker\ningest_repository_scan_worker.go]
  IA[IngestAssetWorker\ningest_asset_worker.go]
  DA[DiscoverAssetWorker\ningest_discover_worker.go]
  AR[AssetRetryWorker\nretry_asset_worker.go]
  RI[ReindexAssetsWorker\nanalysis_indexing_worker.go]

  %% Media pipeline
  M[MetadataWorker\nmedia_metadata_worker.go]
  T[ThumbnailWorker\nmedia_thumbnail_worker.go]
  X[TranscodeWorker\nmedia_transcode_worker.go]

  %% Analysis / enrichment
  PH[ProcessPHashWorker\nanalysis_phash_worker.go]
  DS[DetectStacksWorker\nanalysis_detect_stacks_worker.go]
  LP[LivePhotoMatchWorker\nanalysis_livephoto_worker.go]
  LC[RebuildLocationClustersWorker\nmedia_location_worker.go]

  %% ML pipeline
  CLIP[ProcessClipWorker\nml_clip_worker.go]
  BIO[ProcessBioClipWorker\nml_bioclip_worker.go]
  OCR[ProcessOcrWorker\nml_ocr_worker.go]
  FACE[ProcessFaceWorker\nml_face_worker.go]

  %% Upstream orchestration
  SR -->|enqueue discover_asset| DA
  SR -->|enqueue detect_stacks after scan| DS

  %% Ingest / discovery fan-out
  IA -->|enqueue metadata_asset| M
  IA -->|enqueue thumbnail_asset| T
  IA -->|enqueue transcode_asset for video or audio| X

  DA -->|enqueue metadata_asset| M
  DA -->|enqueue thumbnail_asset| T
  DA -->|enqueue transcode_asset for video or audio| X

  %% Metadata fan-out
  M -->|photo only: GPS present| LC
  M -->|photo only| DS
  M -->|photo/video: content_identifier present| LP

  %% Thumbnail fan-out
  T -->|photo only: pHash fallback| PH
  T -->|photo only| CLIP
  T -->|photo only| BIO
  T -->|photo only| OCR
  T -->|photo only| FACE

  %% Retry dispatcher
  AR -.->|metadata_asset| M
  AR -.->|thumbnail_asset| T
  AR -.->|transcode_asset| X
  AR -.->|process_phash| PH
  AR -.->|process_clip| CLIP
  AR -.->|process_bioclip| BIO
  AR -.->|process_ocr| OCR
  AR -.->|process_face| FACE

  %% Root / leaf workers without downstream edges
  RI
```

## Execution Notes

- `ScanRepositoryWorker` is the highest-level orchestration entry point for repository tree scans. It enqueues `DiscoverAssetWorker` per file and also schedules `DetectStacksWorker` after a scan completes.
- `IngestAssetWorker` and `DiscoverAssetWorker` both converge into the same media pipeline:
  - `MetadataWorker` is always first.
  - `ThumbnailWorker` follows for photos and videos.
  - `TranscodeWorker` follows for videos and audio.
- `MetadataWorker` is the main enrichment fan-out:
  - Photo metadata can trigger `RebuildLocationClustersWorker`, `DetectStacksWorker`, and `LivePhotoMatchWorker`.
  - Video metadata can trigger `LivePhotoMatchWorker` when `content_identifier` exists.
- `ThumbnailWorker` is the image enrichment fan-out:
  - Photos can trigger `ProcessPHashWorker` when thumbnail generation falls back.
  - Photos can also trigger CLIP, BioCLIP, OCR, and Face workers when the ML settings enable them.
- `AssetRetryWorker` is a dispatcher. It does not depend on a single downstream worker; instead, it can re-enqueue any task based on the retry request.

## Idempotence Rules

- `DetectStacksWorker` must tolerate repeated runs. It is safe to run after scans and after metadata extraction because stack creation checks existing membership before inserting.
- `LivePhotoMatchWorker` must tolerate:
  - photo before video
  - video before photo
  - metadata job retries
  - live photo matcher retries
  It uses exact `owner_id + content_identifier` matching and stack-membership checks to avoid duplicate stacks.
- `ThumbnailWorker` and ML workers are queue-level idempotent via River uniqueness plus the workers' own asset-state checks.
- `AssetRetryWorker` is intentionally permissive. It only re-enqueues the tasks requested by the caller, so the downstream workers must remain safe to run repeatedly.
