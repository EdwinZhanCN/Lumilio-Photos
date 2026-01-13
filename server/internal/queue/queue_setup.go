package queue

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// New River Client, add your queue here.
func New(dbpool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	// Queue configurations based on workload characteristics:
	// - process_asset: High throughput, needs more workers for file processing
	// - process_clip: CPU-intensive ML processing, limited workers
	// - process_ocr: Text extraction, moderate workers
	// - process_caption: AI captioning, resource-intensive, limited workers
	// - process_face: Face detection & recognition, CPU/memory intensive, moderate workers
	// - retry_asset: Error recovery, lower priority
	client, err := river.NewClient(riverpgxv5.New(dbpool), &river.Config{
		Schema: "public",
		Queues: map[string]river.QueueConfig{
			// Asset processing queue - handles file uploads and basic processing
			// High throughput, needs multiple workers for concurrent file handling
			"process_asset": {MaxWorkers: 5},

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
