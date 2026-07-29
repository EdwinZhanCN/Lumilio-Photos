package event

import "time"

type Policy struct {
	Version            string
	StrongGap          time.Duration
	MissingLocationGap time.Duration
	NearbyGap          time.Duration
	HardGap            time.Duration
	MaxEventDuration   time.Duration
	NearbyMeters       float64
	MaxTravelSpeedKPH  float64
	ReconcileThreshold float64
}

var V1 = Policy{
	Version:            AlgorithmVersion,
	StrongGap:          90 * time.Minute,
	MissingLocationGap: 3 * time.Hour,
	NearbyGap:          6 * time.Hour,
	HardGap:            12 * time.Hour,
	MaxEventDuration:   36 * time.Hour,
	NearbyMeters:       2000,
	MaxTravelSpeedKPH:  300,
	ReconcileThreshold: .40,
}
