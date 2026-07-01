package dto

import (
	"time"

	"server/internal/service"
)

type PersonSummaryDTO struct {
	PersonID              int32      `json:"person_id"`
	Name                  *string    `json:"name,omitempty"`
	IsConfirmed           bool       `json:"is_confirmed"`
	IsHidden              bool       `json:"is_hidden"`
	HiddenAt              *time.Time `json:"hidden_at,omitempty"`
	MemberCount           int64      `json:"member_count"`
	AssetCount            int64      `json:"asset_count"`
	CoverFaceImagePath    *string    `json:"cover_face_image_path,omitempty"`
	RepresentativeAssetID *string    `json:"representative_asset_id,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type PersonDetailDTO struct {
	PersonSummaryDTO
}

type ListPeopleResponseDTO struct {
	People []PersonSummaryDTO `json:"people"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

type FaceClusterRebuildResponseDTO struct {
	Algorithm       string  `json:"algorithm"`
	RepositoryID    *string `json:"repository_id,omitempty"`
	CandidateFaces  int     `json:"candidate_faces"`
	ClusteredFaces  int     `json:"clustered_faces"`
	NoiseFaces      int     `json:"noise_faces"`
	ClustersCreated int     `json:"clusters_created"`
	ClustersReused  int     `json:"clusters_reused"`
	ClustersTotal   int     `json:"clusters_total"`
	DurationMs      int64   `json:"duration_ms"`
}

type UpdatePersonRequestDTO struct {
	Name string `json:"name" binding:"required"`
}

// PersonFaceDTO is a UI-safe view of one face that belongs to a person. It
// deliberately omits embeddings, bounding boxes, pose angles and demographic
// attributes.
type PersonFaceDTO struct {
	FaceID           int32      `json:"face_id"`
	AssetID          string     `json:"asset_id"`
	Confidence       float32    `json:"confidence"`
	IsRepresentative bool       `json:"is_representative"`
	IsManual         bool       `json:"is_manual"`
	HasCrop          bool       `json:"has_crop"`
	Filename         string     `json:"filename,omitempty"`
	TakenTime        *time.Time `json:"taken_time,omitempty"`
	UploadTime       *time.Time `json:"upload_time,omitempty"`
}

type ListPersonFacesResponseDTO struct {
	Faces  []PersonFaceDTO `json:"faces"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type MergePeopleRequestDTO struct {
	SourcePersonIDs []int32 `json:"source_person_ids" binding:"required"`
}

type MoveFaceRequestDTO struct {
	TargetPersonID int32 `json:"target_person_id" binding:"required"`
}

type SetPersonCoverRequestDTO struct {
	FaceID int32 `json:"face_id" binding:"required"`
}

type SetPersonHiddenRequestDTO struct {
	Hidden bool `json:"hidden"`
}

// PersonCorrectionResponseDTO is the focused result of a correction action. The
// person field is the updated target person, or null when the action emptied
// and removed the source person (for example moving its last face away).
type PersonCorrectionResponseDTO struct {
	Person *PersonDetailDTO `json:"person,omitempty"`
}

func ToPersonFaceDTO(face service.PersonFace) PersonFaceDTO {
	return PersonFaceDTO{
		FaceID:           face.FaceID,
		AssetID:          face.AssetID,
		Confidence:       face.Confidence,
		IsRepresentative: face.IsRepresentative,
		IsManual:         face.IsManual,
		HasCrop:          face.HasCrop,
		Filename:         face.Filename,
		TakenTime:        face.TakenTime,
		UploadTime:       face.UploadTime,
	}
}

func ToPersonSummaryDTO(person service.Person) PersonSummaryDTO {
	return PersonSummaryDTO{
		PersonID:              person.PersonID,
		Name:                  person.Name,
		IsConfirmed:           person.IsConfirmed,
		IsHidden:              person.IsHidden,
		HiddenAt:              person.HiddenAt,
		MemberCount:           person.MemberCount,
		AssetCount:            person.AssetCount,
		CoverFaceImagePath:    person.CoverFaceImagePath,
		RepresentativeAssetID: person.RepresentativeAssetID,
		CreatedAt:             person.CreatedAt,
		UpdatedAt:             person.UpdatedAt,
	}
}

func ToPersonDetailDTO(person service.Person) PersonDetailDTO {
	return PersonDetailDTO{
		PersonSummaryDTO: ToPersonSummaryDTO(person),
	}
}

func ToFaceClusterRebuildResponseDTO(result service.FaceClusterRebuildResult) FaceClusterRebuildResponseDTO {
	return FaceClusterRebuildResponseDTO{
		Algorithm:       result.Algorithm,
		RepositoryID:    result.RepositoryID,
		CandidateFaces:  result.CandidateFaces,
		ClusteredFaces:  result.ClusteredFaces,
		NoiseFaces:      result.NoiseFaces,
		ClustersCreated: result.ClustersCreated,
		ClustersReused:  result.ClustersReused,
		ClustersTotal:   result.ClustersTotal,
		DurationMs:      result.DurationMs,
	}
}
