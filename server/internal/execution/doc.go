// Package execution owns process-wide admission for fine-grained pipeline
// work. Fine-grained steps form an in-process DAG inside one River macro job,
// never child River jobs. [Engine.Run] admits each step through the [Governor]
// against explicit [Resources] — CPU, disk I/O, image codec, video codec,
// inference, and memory — in the step's QoS [Class], using the demand declared
// by the [DemandCatalog]. Runtime diagnostics report execution waits next to
// desired/applied lag and coordinator load.
//
//atlas:group work
package execution
