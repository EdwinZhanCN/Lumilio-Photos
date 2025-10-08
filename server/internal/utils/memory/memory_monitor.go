package memory

import (
	"fmt"
	"log"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

// ChunkConfig represents the dynamic chunk configuration based on system memory
type ChunkConfig struct {
	ChunkSize      int64 `json:"chunk_size"`      // in bytes
	MaxConcurrent  int   `json:"max_concurrent"`  // maximum concurrent uploads
	MemoryBuffer   int64 `json:"memory_buffer"`   // safety buffer in bytes
	UpdateInterval int   `json:"update_interval"` // config cache duration in seconds
}

// MemoryMonitor handles memory-aware configuration management
type MemoryMonitor struct {
	configCache      *ChunkConfig
	configLastUpdate time.Time
	cacheDuration    time.Duration
}

// NewMemoryMonitor creates a new memory monitor instance
func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{
		cacheDuration: 30 * time.Second, // Cache config for 30 seconds
	}
}

// GetOptimalChunkConfig returns the optimal chunk configuration based on available memory
func (m *MemoryMonitor) GetOptimalChunkConfig() (*ChunkConfig, error) {
	// Return cached config if still valid
	if m.configCache != nil && time.Since(m.configLastUpdate) < m.cacheDuration {
		return m.configCache, nil
	}

	// Get current memory information
	vm, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("Failed to get memory info, using default config: %v", err)
		return m.getDefaultConfig(), nil
	}

	// Calculate available memory in MB
	availableMB := int64(vm.Available) / 1024 / 1024

	var chunkSize int64
	var maxConcurrent int

	// Dynamic configuration based on available memory
	switch {
	case availableMB > 4096: // 4GB+ available memory
		chunkSize = 20 * 1024 * 1024 // 20MB
		maxConcurrent = 8
	case availableMB > 2048: // 2GB-4GB available memory
		chunkSize = 10 * 1024 * 1024 // 10MB
		maxConcurrent = 5
	case availableMB > 1024: // 1GB-2GB available memory
		chunkSize = 5 * 1024 * 1024 // 5MB
		maxConcurrent = 3
	default: // <1GB available memory
		chunkSize = 2 * 1024 * 1024 // 2MB
		maxConcurrent = 2
	}

	// Calculate safety buffer (10% of available memory)
	memoryBuffer := int64(float64(vm.Available) * 0.1)

	config := &ChunkConfig{
		ChunkSize:      chunkSize,
		MaxConcurrent:  maxConcurrent,
		MemoryBuffer:   memoryBuffer,
		UpdateInterval: 30,
	}

	// Update cache
	m.configCache = config
	m.configLastUpdate = time.Now()

	log.Printf("Memory monitor: available=%dMB, chunk_size=%dMB, max_concurrent=%d",
		availableMB, chunkSize/1024/1024, maxConcurrent)

	return config, nil
}

// CanAcceptNewUpload checks if system has enough memory to accept a new upload
func (m *MemoryMonitor) CanAcceptNewUpload(fileSize int64) (bool, string) {
	config, err := m.GetOptimalChunkConfig()
	if err != nil {
		return true, "memory check unavailable" // Fallback to allow upload
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return true, "memory check failed" // Fallback to allow upload
	}

	// Estimate memory requirement: file size * 2 (for processing buffers)
	estimatedMemory := fileSize * 2

	availableMemory := int64(vm.Available)
	requiredMemory := estimatedMemory + config.MemoryBuffer

	if availableMemory < requiredMemory {
		reason := fmt.Sprintf("insufficient memory: available=%dMB, required=%dMB",
			availableMemory/1024/1024, requiredMemory/1024/1024)
		return false, reason
	}

	return true, "sufficient memory available"
}

// GetMemoryInfo returns current memory usage information
func (m *MemoryMonitor) GetMemoryInfo() (*mem.VirtualMemoryStat, error) {
	return mem.VirtualMemory()
}

// getDefaultConfig returns a safe default configuration
func (m *MemoryMonitor) getDefaultConfig() *ChunkConfig {
	return &ChunkConfig{
		ChunkSize:      5 * 1024 * 1024,   // 5MB
		MaxConcurrent:  3,                 // 3 concurrent uploads
		MemoryBuffer:   100 * 1024 * 1024, // 100MB buffer
		UpdateInterval: 30,                // 30 seconds
	}
}

// SetCacheDuration allows customizing the cache duration
func (m *MemoryMonitor) SetCacheDuration(duration time.Duration) {
	m.cacheDuration = duration
}
