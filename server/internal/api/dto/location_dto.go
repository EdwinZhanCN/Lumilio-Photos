package dto

import "time"

type LocationClusterDTO struct {
	ClusterID         string     `json:"cluster_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID      string     `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Geohash           string     `json:"geohash" example:"9q8yyk8"`
	Precision         int32      `json:"precision" example:"7"`
	CentroidLatitude  float64    `json:"centroid_latitude" example:"37.7749"`
	CentroidLongitude float64    `json:"centroid_longitude" example:"-122.4194"`
	PhotoCount        int32      `json:"photo_count" example:"42"`
	Label             *string    `json:"label,omitempty" example:"San Francisco, California, United States"`
	Country           *string    `json:"country,omitempty" example:"United States"`
	Region            *string    `json:"region,omitempty" example:"California"`
	City              *string    `json:"city,omitempty" example:"San Francisco"`
	Provider          *string    `json:"provider,omitempty" example:"nominatim"`
	GeocodeStatus     string     `json:"geocode_status" example:"resolved"`
	GeocodedAt        *time.Time `json:"geocoded_at,omitempty" example:"2026-02-10T12:00:00Z"`
}

type LocationClusterListResponseDTO struct {
	Clusters []LocationClusterDTO `json:"clusters"`
	Total    *int                 `json:"total,omitempty" example:"150"`
	Limit    int                  `json:"limit" example:"100"`
	Offset   int                  `json:"offset" example:"0"`
}

type RebuildLocationClustersRequestDTO struct {
	RepositoryID string `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type RebuildLocationClustersResponseDTO struct {
	Status       string  `json:"status" example:"queued"`
	Message      string  `json:"message" example:"Location cluster rebuild queued successfully"`
	JobID        int64   `json:"job_id" example:"123"`
	RepositoryID *string `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}
