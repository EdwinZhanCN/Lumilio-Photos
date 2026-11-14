package app

import (
	"fx/processors"
	"fx/river"
)

// Register wires the workers; the pipeline runtime may.
func Register(w *river.Workers, p *processors.AssetProcessor) error {
	river.AddWorker[int](w)
	return p.ComputeMetadataTask()
}
