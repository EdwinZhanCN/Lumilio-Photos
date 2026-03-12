package dto

import (
	"time"

	"server/internal/service"
)

type PersonSummaryDTO struct {
	PersonID              int32     `json:"person_id"`
	Name                  *string   `json:"name,omitempty"`
	IsConfirmed           bool      `json:"is_confirmed"`
	MemberCount           int64     `json:"member_count"`
	AssetCount            int64     `json:"asset_count"`
	CoverFaceImagePath    *string   `json:"cover_face_image_path,omitempty"`
	RepresentativeAssetID *string   `json:"representative_asset_id,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
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

type UpdatePersonRequestDTO struct {
	Name string `json:"name" binding:"required"`
}

func ToPersonSummaryDTO(person service.Person) PersonSummaryDTO {
	return PersonSummaryDTO{
		PersonID:              person.PersonID,
		Name:                  person.Name,
		IsConfirmed:           person.IsConfirmed,
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
