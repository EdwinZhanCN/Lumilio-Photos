package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// manualFaceMatchIoU is how much a re-detected face must overlap a manually
// assigned one to inherit the assignment (#223 in-place carry-over).
const manualFaceMatchIoU = 0.5

// manualFaceAssignment is a user's person assignment on a face that a
// re-detection is about to replace.
type manualFaceAssignment struct {
	clusterID int32
	box       dbtypes.FaceBoundingBox
}

func captureManualFaceAssignments(ctx context.Context, queries *repo.Queries, assetID uuid.UUID) ([]manualFaceAssignment, error) {
	rows, err := queries.ListManualFaceAssignmentsForAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("capture manual face assignments: %w", err)
	}
	assignments := make([]manualFaceAssignment, 0, len(rows))
	for _, row := range rows {
		var box dbtypes.FaceBoundingBox
		if err := json.Unmarshal(row.BoundingBox, &box); err != nil {
			continue
		}
		assignments = append(assignments, manualFaceAssignment{clusterID: row.ClusterID, box: box})
	}
	return assignments, nil
}

// faceBoxIoU is the intersection over union of two face boxes.
func faceBoxIoU(a, b dbtypes.FaceBoundingBox) float64 {
	width := min(a.X2, b.X2) - max(a.X1, b.X1)
	height := min(a.Y2, b.Y2) - max(a.Y1, b.Y1)
	if width <= 0 || height <= 0 {
		return 0
	}
	intersection := float64(width) * float64(height)
	areaA := float64(a.X2-a.X1) * float64(a.Y2-a.Y1)
	areaB := float64(b.X2-b.X1) * float64(b.Y2-b.Y1)
	union := areaA + areaB - intersection
	if union <= 0 {
		return 0
	}
	return intersection / union
}

// reapplyManualFaceAssignments carries a user's person assignments over to
// the faces a re-detection produced, after the Asset's content changed in
// place. Each assignment goes to the re-detected face it overlaps most: with
// IoU of at least 0.5 it stays a manual assignment; with any smaller overlap
// the face joins the person as an unconfirmed (automatic) member the user can
// confirm or move, so the assignment is surfaced rather than dropped. A face
// takes at most one assignment, the best-overlapping pairs first. It returns
// the clusters it touched.
func reapplyManualFaceAssignments(ctx context.Context, queries *repo.Queries, assignments []manualFaceAssignment, faces []repo.FaceItem) ([]int32, error) {
	type candidate struct {
		assignment int
		face       int
		iou        float64
	}
	boxes := make([]*dbtypes.FaceBoundingBox, len(faces))
	for index, face := range faces {
		var box dbtypes.FaceBoundingBox
		if json.Unmarshal(face.BoundingBox, &box) == nil {
			boxes[index] = &box
		}
	}
	var candidates []candidate
	for a, assignment := range assignments {
		for f, box := range boxes {
			if box == nil {
				continue
			}
			if iou := faceBoxIoU(assignment.box, *box); iou > 0 {
				candidates = append(candidates, candidate{assignment: a, face: f, iou: iou})
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].iou > candidates[j].iou })
	assignedFace := make(map[int]bool)
	placed := make(map[int]bool)
	var touched []int32
	for _, match := range candidates {
		if assignedFace[match.face] || placed[match.assignment] {
			continue
		}
		assignedFace[match.face] = true
		placed[match.assignment] = true
		clusterID := assignments[match.assignment].clusterID
		manual := match.iou >= manualFaceMatchIoU
		confidence := 1.0
		if !manual {
			confidence = match.iou
		}
		if _, err := queries.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
			ClusterID: clusterID, FaceID: faces[match.face].ID,
			SimilarityScore: confidence, Confidence: confidence, IsManual: manual,
		}); err != nil {
			return touched, fmt.Errorf("carry person assignment over to face %d: %w", faces[match.face].ID, err)
		}
		touched = append(touched, clusterID)
	}
	return touched, nil
}
