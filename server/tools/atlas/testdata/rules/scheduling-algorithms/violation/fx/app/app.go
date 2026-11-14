package app

import (
	"fx/processors"
	"fx/river"
)

// Wire runs scheduling work in the composition root.
func Wire(w *river.Workers, p *processors.AssetProcessor) error {
	river.AddWorker[string](w)
	return p.ComputeMetadataTask()
}
