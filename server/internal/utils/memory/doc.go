// Package memory adapts upload behaviour to host memory. [MemoryMonitor]
// derives a [ChunkConfig] (chunk size and concurrency) from current available
// memory and answers [MemoryMonitor.CanAcceptNewUpload] before the HTTP layer
// admits another large upload.
//
//atlas:group foundation
package memory
