# Smart Batch Sizing System

## Overview

The Smart Batch Sizing System automatically optimizes batch sizes for image processing operations based on device capabilities, processing feedback, and user preferences. This system replaces fixed batch sizes with dynamic, adaptive sizing to improve performance across different devices.

## Key Features

### 1. Device Capability Detection
- **CPU Cores**: Detects available CPU cores using `navigator.hardwareConcurrency`
- **Memory**: Estimates available memory using the experimental `performance.memory` API
- **Device Classification**: Automatically classifies devices as low-end, mobile, or high-end
- **Concurrency Limits**: Calculates optimal concurrent operation limits

### 2. Adaptive Batch Sizing
- **Performance Feedback**: Learns from processing time and error rates
- **Dynamic Adjustment**: Increases batch sizes for fast operations, decreases for slow ones
- **Memory Pressure Response**: Reduces batch sizes when memory usage is high
- **Priority Queuing**: Prioritizes user-visible operations (thumbnails) with larger batch sizes

### 3. User Performance Preferences
- **Memory Saver**: Optimizes for low memory usage (0.6x batch sizes)
- **Balanced**: Default balanced approach (1.0x batch sizes)
- **Speed Optimized**: Maximizes processing speed (1.5x batch sizes)
- **Adaptive**: Automatically adjusts based on device capabilities

### 4. Real-time Metrics Collection
- **Processing Time**: Tracks batch processing duration
- **Error Rates**: Monitors operation success/failure rates
- **Memory Usage**: Tracks memory consumption when available
- **Throughput**: Measures files processed per second

## Usage

### Basic Usage in Hooks

```typescript
import { getOptimalBatchSize, recordProcessingMetrics, ProcessingPriority } from '@/utils/smartBatchSizing';

// Get optimal batch size
const batchSize = getOptimalBatchSize('thumbnail', files.length, ProcessingPriority.CRITICAL);

// Process in batches
for (let i = 0; i < files.length; i += batchSize) {
  const batch = files.slice(i, i + batchSize);
  const startTime = performance.now();
  
  try {
    // Process batch...
    const processingTime = performance.now() - startTime;
    
    // Record successful metrics
    recordProcessingMetrics({
      operationType: 'thumbnail',
      batchSize: batch.length,
      processingTimeMs: processingTime,
      filesProcessed: batch.length,
      avgTimePerFile: processingTime / batch.length,
      success: true,
      errorRate: 0,
    });
  } catch (error) {
    // Record failure metrics
    recordProcessingMetrics({
      operationType: 'thumbnail',
      batchSize: batch.length,
      processingTimeMs: performance.now() - startTime,
      filesProcessed: 0,
      avgTimePerFile: 0,
      success: false,
      errorRate: 1.0,
    });
  }
}
```

### User Preferences

```typescript
import { usePerformancePreferences, PerformanceProfile } from '@/utils/performancePreferences';

function PerformanceSettings() {
  const { preferences, updatePreferences } = usePerformancePreferences();
  
  const handleProfileChange = (profile: PerformanceProfile) => {
    updatePreferences({ profile });
  };
  
  // UI components...
}
```

## Operation Types and Default Configurations

| Operation | Min Batch | Max Batch | Target Time | Priority Multiplier |
|-----------|-----------|-----------|-------------|-------------------|
| thumbnail | 2 | 20 | 3000ms | 1.5x |
| border | 1 | 10 | 5000ms | 1.2x |
| hash | 5 | 50 | 2000ms | 1.0x |
| exif | 3 | 30 | 4000ms | 1.1x |
| export | 1 | 5 | 10000ms | 2.0x |

## Processing Priorities

- **LOW (1)**: Background operations
- **NORMAL (2)**: Standard processing
- **HIGH (3)**: Important operations
- **CRITICAL (4)**: User-visible operations (thumbnails, exports)

## Memory Pressure Detection

The system monitors memory usage and automatically reduces batch sizes when:
- Memory usage exceeds 80% of available heap
- Device is classified as low-end
- User has enabled strict memory limits

## Performance Profiles

### Memory Saver (0.6x multiplier)
- Reduces all batch sizes by 40%
- Strict memory constraints (0.5x limit)
- Best for older devices or limited memory

### Balanced (1.0x multiplier)
- Uses default batch sizes
- Moderate memory constraints (0.8x limit)
- Good general-purpose setting

### Speed Optimized (1.5x multiplier)
- Increases batch sizes by 50%
- Relaxed memory constraints (1.2x limit)
- Best for high-end devices with plenty of RAM

### Adaptive (1.0x multiplier)
- Automatically adjusts based on device capabilities
- Conservative memory approach (0.8x limit)
- Recommended for most users

## Files Modified

### Core System
- `web/src/utils/smartBatchSizing.ts` - Main batch sizing logic
- `web/src/utils/performancePreferences.ts` - User preference management

### Updated Hooks
- `web/src/hooks/util-hooks/useGenerateBorder.tsx`
- `web/src/hooks/util-hooks/useGenerateThumbnail.tsx`
- `web/src/hooks/util-hooks/useGenerateHashcode.tsx`
- `web/src/hooks/util-hooks/useExtractExifdata.tsx`
- `web/src/hooks/util-hooks/useExportImage.tsx`

### Settings UI
- `web/src/features/settings/components/Tabs/PerformanceSettings.tsx`
- `web/src/features/settings/components/SettingsTab.tsx`

## Testing

Comprehensive tests cover:
- Device capability detection
- Batch size adaptation based on performance metrics
- Memory pressure detection
- User preference management
- Edge cases and error handling

Run tests with: `npm test src/utils`

## Browser Compatibility

- **Modern Browsers**: Full feature support with memory API
- **Older Browsers**: Fallback to conservative estimates
- **Node.js**: Safe fallbacks for server-side rendering

## Performance Impact

- **Minimal Overhead**: ~1-2ms per batch size calculation
- **Memory Efficient**: Uses Map-based caching with size limits
- **Storage**: User preferences stored in localStorage (~1KB)
- **No Network Requests**: All processing is client-side

## Future Enhancements

- Integration with Web Workers for memory monitoring
- Machine learning-based batch size prediction
- Cross-session performance learning
- Device-specific configuration profiles
- Real-time memory usage visualization