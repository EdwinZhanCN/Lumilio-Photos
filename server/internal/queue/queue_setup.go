package queue

import (
	"runtime"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// New River Client, add your queue here.
func New(dbpool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	// Queue configurations based on workload characteristics:
	// - ingest_asset: High throughput, staging validation + record creation
	// - metadata_asset: EXIF/ffmpeg probing, moderate throughput
	// - thumbnail_asset: CPU-bound thumbnail generation, scaled to available cores
	// - transcode_asset: Serialized video/audio transcoding, single worker to avoid resource contention
	// - process_clip: CPU-intensive ML processing, limited workers
	// - process_ocr: Text extraction, moderate workers
	// - process_caption: AI captioning, resource-intensive, limited workers
	// - process_face: Face detection & recognition, CPU/memory intensive, moderate workers
	// - retry_asset: Error recovery, lower priority
	client, err := river.NewClient(riverpgxv5.New(dbpool), &river.Config{
		Schema: "public",
		Queues: map[string]river.QueueConfig{
			// Ingest queue - staging validation and asset record creation
			"ingest_asset": {MaxWorkers: 50},

			// Metadata queue - EXIF/ffmpeg probing
			"metadata_asset": {MaxWorkers: 20},

			// Thumbnail queue - CPU-bound; match available cores
			"thumbnail_asset": {MaxWorkers: runtime.NumCPU()},

			// Transcode queue - serialized video/audio transcoding
			"transcode_asset": {MaxWorkers: 1},

			// CLIP embedding queue - generates image embeddings and classifications
			// CPU-intensive ML processing, limit workers to prevent resource exhaustion
			"process_clip": {MaxWorkers: 2},

			// OCR text extraction queue - processes images for text content
			// Moderate resource usage, can handle concurrent processing
			"process_ocr": {MaxWorkers: 3},

			// AI captioning queue - generates image descriptions using VLM
			// Resource-intensive, limit workers to manage memory and API usage
			"process_caption": {MaxWorkers: 1},

			// Face detection and recognition queue - processes faces in images
			// CPU and memory intensive due to face detection and embedding generation
			// Moderate workers to balance performance and resource usage
			"process_face": {MaxWorkers: 2},

			// Asset retry queue - handles failed processing jobs
			// Lower priority, fewer workers needed
			"retry_asset": {MaxWorkers: 2},
		},
		Workers: workers,
	})
	return client, err
}
