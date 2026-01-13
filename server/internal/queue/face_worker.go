package queue

import (
	"context"
	"fmt"
	"server/internal/queue/jobs"
	"server/internal/service"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

// ProcessFaceArgs is the job payload.
type ProcessFaceArgs = jobs.ProcessFaceArgs

// ProcessFaceWorker handles face detection and recognition for assets
type ProcessFaceWorker struct {
	river.WorkerDefaults[ProcessFaceArgs]

	FaceService service.FaceService
	LumenService service.LumenService
}

func (w *ProcessFaceWorker) Work(ctx context.Context, job *river.Job[ProcessFaceArgs]) error {
	args := job.Args
	assetID := args.AssetID

	// Convert UUID to pgtype.UUID for database operations
	pgUUID := pgtype.UUID{}
	if err := pgUUID.Scan(assetID.String()); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	// Start timing the face processing
	startTime := time.Now()

	// Perform face detection using LumenService
	faceV1, err := w.LumenService.FaceDetectEmbed(ctx, args.ImageData)
	if err != nil {
		return fmt.Errorf("failed to perform face detection: %w", err)
	}

	// Calculate processing time
	processingTimeMs := int(time.Since(startTime).Milliseconds())

	// Save face results using FaceService (FaceService will handle conversion from FaceV1 to database format)
	err = w.FaceService.SaveFaceResults(ctx, pgUUID, faceV1, processingTimeMs)
	if err != nil {
		return fmt.Errorf("failed to save face results: %w", err)
	}

	// If faces were detected and have embeddings, attempt to find similar faces for clustering
	if len(faceV1.Faces) > 0 {
		go func() {
			// Use a background context for clustering since the main job is done
			bgCtx := context.Background()

			for i, face := range faceV1.Faces {
				if face.Embedding != nil && len(face.Embedding) > 0 {
					// Find similar faces for clustering/recognition
					// Note: We use a temporary face ID based on index since we don't have database IDs yet
					tempFaceID := int32(i + 1000) // Use offset to avoid conflicts
					similarFaces, err := w.FaceService.FindSimilarFaces(
						bgCtx,
						face.Embedding,
						tempFaceID,
						0.7, // Minimum similarity threshold
						10,  // Limit results
					)
					if err != nil {
						fmt.Printf("Failed to find similar faces for face clustering: %v\n", err)
						continue
					}

					// If we found similar faces, we could trigger clustering logic here
					if len(similarFaces) > 0 {
						fmt.Printf("Found %d similar faces for clustering\n", len(similarFaces))
						// TODO: Implement automatic clustering or manual review trigger
					}
				}
			}
		}()
	}

	return nil
}

// No additional types needed - using types.FaceV1 directly from lumen-sdk