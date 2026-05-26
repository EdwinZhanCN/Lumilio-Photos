package dto

import "server/internal/service"

type SpeciesReferenceResponseDTO struct {
	Provider         string `json:"provider" example:"inaturalist"`
	Query            string `json:"query" example:"Rucervus duvaucelii"`
	TaxonID          int    `json:"taxon_id" example:"75046"`
	ScientificName   string `json:"scientific_name,omitempty" example:"Rucervus duvaucelii"`
	CommonName       string `json:"common_name,omitempty" example:"Barasingha"`
	WikipediaSummary string `json:"wikipedia_summary,omitempty" example:"The barasingha, also called swamp deer, is a deer species distributed in the Indian subcontinent."`
	WikipediaURL     string `json:"wikipedia_url,omitempty" example:"https://en.wikipedia.org/wiki/Rucervus_duvaucelii"`
	ReferenceURL     string `json:"reference_url,omitempty" example:"https://www.inaturalist.org/taxa/75046"`
	ImageURL         string `json:"image_url,omitempty" example:"https://inaturalist-open-data.s3.amazonaws.com/photos/231650420/large.jpeg"`
	ImageAttribution string `json:"image_attribution,omitempty" example:"(c) Ramesh Shenai Jr., some rights reserved (CC BY), uploaded by Ramesh Shenai Jr."`
	ImageLicense     string `json:"image_license,omitempty" example:"cc-by"`
	ImageSourceURL   string `json:"image_source_url,omitempty" example:"https://www.inaturalist.org/photos/231650420"`
}

func ToSpeciesReferenceResponseDTO(ref *service.SpeciesReference) SpeciesReferenceResponseDTO {
	if ref == nil {
		return SpeciesReferenceResponseDTO{}
	}

	return SpeciesReferenceResponseDTO{
		Provider:         ref.Provider,
		Query:            ref.Query,
		TaxonID:          ref.TaxonID,
		ScientificName:   ref.ScientificName,
		CommonName:       ref.CommonName,
		WikipediaSummary: ref.WikipediaSummary,
		WikipediaURL:     ref.WikipediaURL,
		ReferenceURL:     ref.ReferenceURL,
		ImageURL:         ref.ImageURL,
		ImageAttribution: ref.ImageAttribution,
		ImageLicense:     ref.ImageLicense,
		ImageSourceURL:   ref.ImageSourceURL,
	}
}
