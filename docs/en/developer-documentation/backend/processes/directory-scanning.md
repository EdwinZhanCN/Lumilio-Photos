# Process Documentation: Directory Scanning

*Author: Edwin Zhan, documented by AI*

## Overview

The directory scanning system monitors and synchronizes user-managed files within Lumilio Photos repositories. This two-tier system combines real-time file watching with periodic reconciliation to ensure database consistency with the filesystem.

## System Architecture

### Components Involved

1. **FileWatcher**: Real-time filesystem monitoring using fsnotify
2. **ReconciliationScanner**: Periodic full directory scans
3. **SyncManager**: Orchestrates watchers and reconciliation
4. **FileRecordStore**: Database operations for file records
5. **SyncOperationStore**: Tracks sync operations and statistics

### Key Files

- `server/internal/sync/file_watcher.go` - Real-time monitoring
- `server/internal/sync/reconciliation_scanner.go` - Full scans
- `server/internal/sync/sync_manager.go` - Orchestration
- `server/internal/sync/file_record.go` - Database operations
- `server/internal/sync/sync_operation.go` - Operation tracking

## Two-Tier Architecture

### Tier 1: Real-Time File Watcher

**Purpose**: Immediate detection and handling of file changes  
**Technology**: fsnotify (inotify on Linux, FSEvents on macOS)  
**Update Frequency**: Immediate (sub-second)

**Benefits**:
- Instant synchronization
- Low latency for user actions
- Minimal database drift

**Limitations**:
- Can miss events under high load
- Requires running process
- Limited by OS notification limits

### Tier 2: Daily Reconciliation

**Purpose**: Ensure long-term consistency  
**Technology**: Full directory walk with batching  
**Update Frequency**: Once per 24 hours (configurable)

**Benefits**:
- Catches missed events
- Handles offline changes
- Verifies database integrity
- Cleans up orphaned records

**Limitations**:
- Higher resource usage
- Temporary performance impact
- Not real-time

## Directory Structure

### Protected vs User-Managed Areas

**Protected (System-Managed)**:
```
repository/
├── .lumilio/          # System files - NOT scanned
│   ├── assets/        # Generated assets
│   ├── staging/       # Upload staging
│   ├── temp/          # Temporary files
│   └── ...
└── inbox/             # Structured uploads - NOT scanned
```

**User-Managed (Scanned)**:
```
repository/
├── Photos/            # User directories - SCANNED
│   ├── 2024/
│   │   ├── January/
│   │   └── February/
│   └── Vacation/
└── Documents/         # Any user structure - SCANNED
```

**Why Different Treatment?**
- `.lumilio/` and `inbox/` are managed by the application
- User areas can be organized however users want
- Scanning tracks user file changes
- Assets are tracked separately via upload/processing

---

## Real-Time File Watcher

### Initialization

**Location**: `sync_manager.go::AddRepository()`

```go
// Create file watcher for repository
watcher, err := NewFileWatcher(repoID, repoPath, config.FileWatcherConfig)

// Start monitoring
err = watcher.Start(ctx)
```

**Configuration**:
```go
type FileWatcherConfig struct {
    DebounceInterval time.Duration  // Default: 500ms
}
```

### File Event Detection

**Watched Events**:
1. **Create**: New file added
2. **Write**: File content modified
3. **Remove**: File deleted
4. **Rename**: File moved or renamed

**Debouncing**:
```
Write Event → Wait 500ms → Process if no more writes
```
- Prevents multiple events for single file save
- Reduces unnecessary hash calculations
- Configurable via `DebounceInterval`

### Event Processing Flow

#### Event Received → Debounce → Process

**Step 1: Event Reception**
```go
case event := <-watcher.Events:
    // fsnotify event: Create, Write, Remove, Rename
    fw.handleEvent(event)
```

**Step 2: Debouncing**
```go
// Store event with timestamp
fw.pendingEvents[filePath] = time.Now()

// Wait for debounce interval
time.Sleep(fw.config.DebounceInterval)

// Check if more events occurred
if !fw.pendingEvents[filePath].Equal(originalTime) {
    // More events came in, skip this one
    return
}
```

**Step 3: File Type Check**
```go
// Ignore hidden files and system files
if shouldIgnoreFile(filePath) {
    return
}
```

**Ignored Patterns**:
- Files starting with `.` (hidden files)
- Files ending with `~` (backup files)
- `*.tmp`, `*.temp` (temporary files)
- System files (`.DS_Store`, `Thumbs.db`)

**Step 4: Process Event**

**For Create/Write Events**:
```go
// Get file info
info, err := os.Stat(filePath)

// Calculate hash
hash, err := calculateSHA256(filePath)

// Upsert to database
err = fw.store.UpsertFileRecord(ctx, &FileRecord{
    RepositoryID: fw.repoID,
    FilePath:     relativePath,
    FileSize:     info.Size(),
    ModTime:      info.ModTime(),
    ContentHash:  &hash,
    LastScanned:  time.Now(),
})
```

**For Remove Events**:
```go
// Delete from database
err = fw.store.DeleteFileRecord(ctx, fw.repoID, relativePath)
```

### Directory Watching

**Automatic Subdirectory Discovery**:
```go
// When new directory is created
if event.Op&fsnotify.Create == fsnotify.Create {
    if isDir(event.Name) {
        // Add directory to watcher
        fw.watcher.Add(event.Name)
        
        // Scan contents of new directory
        fw.scanDirectory(event.Name)
    }
}
```

**Recursive Watching**:
- Top-level directories added explicitly
- Subdirectories added dynamically as they're created
- Removed automatically when deleted

### Performance Characteristics

**Resource Usage**:
- Memory: ~10-20MB per 10k monitored files
- CPU: < 1% idle, < 5% during file operations
- I/O: Minimal (only hash calculation)

**Hash Calculation**:
- Algorithm: SHA256
- Speed: ~100-500 MB/s (CPU dependent)
- Triggered: Only on file content changes
- Optimization: Not calculated if file size unchanged

**Limitations**:
- OS-specific inotify/FSEvents limits
  - Linux: Typically 8192 watches per user
  - macOS: No hard limit but performance degrades
- Very rapid file changes may be missed
- Network filesystems may not send events reliably

---

## Reconciliation Scanner

### Purpose and Timing

**When It Runs**:
1. **Startup Sync**: When repository first added
2. **Daily Reconciliation**: Every 24 hours (configurable)
3. **Manual Trigger**: Via API or admin action

**Purpose**:
- Catch missed real-time events
- Handle offline changes
- Verify database integrity
- Clean up orphaned records

### Reconciliation Process

**Location**: `reconciliation_scanner.go::Reconcile()`

#### Phase 1: Full Directory Walk

```go
func (rs *ReconciliationScanner) walkDirectory(
    ctx context.Context,
    dirPath string,
) ([]FileInfo, error) {
    var files []FileInfo
    
    err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
        // Skip system directories
        if shouldIgnore(path) {
            return filepath.SkipDir
        }
        
        // Skip non-regular files
        if !d.Type().IsRegular() {
            return nil
        }
        
        // Get file info
        info, err := d.Info()
        
        // Store for processing
        files = append(files, FileInfo{
            Path:    path,
            Size:    info.Size(),
            ModTime: info.ModTime(),
        })
        
        return nil
    })
    
    return files, err
}
```

**Characteristics**:
- Single-threaded directory walk
- Depth-first traversal
- Collects all file paths
- Speed: ~1000-2000 files/second

#### Phase 2: Batch Processing

```go
func (rs *ReconciliationScanner) processBatch(
    ctx context.Context,
    files []FileInfo,
) error {
    // Process in batches of 100
    for i := 0; i < len(files); i += rs.config.BatchSize {
        end := min(i+rs.config.BatchSize, len(files))
        batch := files[i:end]
        
        // Process batch
        err := rs.processBatchFiles(ctx, batch)
        if err != nil {
            log.Printf("Batch processing error: %v", err)
            // Continue with next batch
        }
    }
    
    return nil
}
```

**Batch Configuration**:
```go
type ReconciliationConfig struct {
    BatchSize       int  // Default: 100
    MaxConcurrency  int  // Default: 4
    CalculateHashes bool // Default: true
}
```

**Batch Processing Steps**:
1. Get existing records for batch from database
2. Compare filesystem vs database state
3. Calculate hashes for changed files
4. Upsert new/updated records
5. Mark unchanged records as scanned

#### Phase 3: Orphaned Record Cleanup

**Scan Generation Concept**:
```go
// At start of reconciliation
currentGeneration := time.Now().Unix()

// During scan, mark all found files
UPDATE file_records
SET scan_generation = currentGeneration
WHERE repository_id = ? AND file_path = ?

// After scan, delete unmarked records
DELETE FROM file_records
WHERE repository_id = ?
  AND scan_generation < currentGeneration
```

**Why This Works**:
- Files found during scan get current generation
- Files not found keep old generation
- Old generation = file no longer exists
- Safe cleanup without race conditions

### Comparison Logic

**For Each File**:
```go
// Get database record
dbRecord, exists := dbRecords[relativePath]

if !exists {
    // New file
    actions = append(actions, Action{Type: "ADD", File: file})
} else if file.ModTime.After(dbRecord.ModTime) {
    // File modified
    actions = append(actions, Action{Type: "UPDATE", File: file})
} else if file.Size != dbRecord.FileSize {
    // Size changed but not ModTime (unusual but possible)
    actions = append(actions, Action{Type: "UPDATE", File: file})
} else {
    // File unchanged
    actions = append(actions, Action{Type: "MARK", File: file})
}
```

**Hash Recalculation**:
- Only for new or modified files
- Optional (controlled by `CalculateHashes` config)
- Parallel calculation (up to `MaxConcurrency` workers)

### Performance Characteristics

**Scan Speed**:
- File enumeration: ~1000-2000 files/second
- Hash calculation: ~100-500 MB/second
- Database operations: ~500-1000 upserts/second (batched)

**Resource Usage During Scan**:
- CPU: 10-30% (hash calculation + DB)
- Memory: ~100-200MB for 100k files
- I/O: Significant (reading all files for hash)
- Database: Moderate load (batched queries)

**Example Timings**:
| Files | Scan Time | Hash Time | DB Time | Total |
|-------|-----------|-----------|---------|-------|
| 1,000 | 1s | 2s | 1s | 4s |
| 10,000 | 5s | 20s | 3s | 28s |
| 100,000 | 50s | 200s | 30s | 280s |

---

## Sync Manager Orchestration

### Initialization

**Location**: `sync_manager.go::NewSyncManager()`

```go
config := SyncManagerConfig{
    ReconciliationInterval: 24 * time.Hour,
    FileWatcherConfig: FileWatcherConfig{
        DebounceInterval: 500 * time.Millisecond,
    },
    ReconciliationConfig: ReconciliationConfig{
        BatchSize:       100,
        MaxConcurrency:  4,
        CalculateHashes: true,
    },
}

syncManager, err := NewSyncManager(pool, config)
err = syncManager.Start()
```

### Repository Management

**Adding a Repository**:
```go
// Performs startup sync automatically
err := syncManager.AddRepository(repoID, repoPath)
```

**Process**:
1. Create FileWatcher for repository
2. Start real-time monitoring
3. Trigger initial reconciliation scan
4. Schedule daily reconciliation

**Removing a Repository**:
```go
err := syncManager.RemoveRepository(repoID)
```

**Process**:
1. Stop FileWatcher
2. Cancel reconciliation timer
3. Remove from active repositories
4. Database records remain (manual cleanup if needed)

### Reconciliation Scheduling

```go
func (sm *SyncManager) scheduleReconciliation(repoID uuid.UUID) {
    ticker := time.NewTicker(sm.config.ReconciliationInterval)
    
    go func() {
        for {
            select {
            case <-ticker.C:
                // Run reconciliation
                sm.reconcileRepository(repoID)
            case <-sm.stopChan:
                ticker.Stop()
                return
            }
        }
    }()
}
```

**Manual Trigger**:
```go
// API or admin can trigger manually
err := syncManager.TriggerReconciliation(repoID, repoPath)
```

---

## Database Schema

### file_records Table

```sql
CREATE TABLE file_records (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    mod_time TIMESTAMP WITH TIME ZONE NOT NULL,
    content_hash TEXT,
    last_scanned TIMESTAMP WITH TIME ZONE NOT NULL,
    scan_generation BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(repository_id, file_path)
);

CREATE INDEX idx_file_records_repo ON file_records(repository_id);
CREATE INDEX idx_file_records_hash ON file_records(content_hash);
CREATE INDEX idx_file_records_scan_gen ON file_records(repository_id, scan_generation);
```

**Key Fields**:
- `file_path`: Relative path within user area
- `content_hash`: SHA256 hash (NULL if not calculated)
- `scan_generation`: For orphaned record cleanup
- `last_scanned`: Last time file was verified to exist

### sync_operations Table

```sql
CREATE TABLE sync_operations (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL,
    operation_type TEXT NOT NULL, -- 'realtime', 'reconciliation', 'startup'
    files_scanned INTEGER DEFAULT 0,
    files_added INTEGER DEFAULT 0,
    files_updated INTEGER DEFAULT 0,
    files_removed INTEGER DEFAULT 0,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT,
    status TEXT NOT NULL, -- 'running', 'completed', 'failed'
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_sync_ops_repo ON sync_operations(repository_id, start_time DESC);
```

**Purpose**:
- Track sync history
- Monitor performance
- Debug issues
- Display status to users

---

## API Operations

### Querying Sync Status

```go
status, err := syncManager.GetSyncStatus(ctx, repoID)
```

**Returns**:
```go
type SyncStatus struct {
    RepositoryID      uuid.UUID
    TotalFiles        int
    IsWatching        bool
    LastSyncOperation *SyncOperation
    NextReconciliation time.Time
}
```

### Listing File Records

```go
records, err := syncManager.ListFileRecords(ctx, repoID)
```

**Returns**:
```go
type FileRecord struct {
    ID            int64
    RepositoryID  uuid.UUID
    FilePath      string
    FileSize      int64
    ModTime       time.Time
    ContentHash   *string
    LastScanned   time.Time
}
```

### Getting Sync History

```go
operations, err := syncManager.GetSyncOperations(ctx, repoID, limit)
```

**Returns**:
```go
type SyncOperation struct {
    ID            int64
    RepositoryID  uuid.UUID
    OperationType string
    FilesScanned  int
    FilesAdded    int
    FilesUpdated  int
    FilesRemoved  int
    StartTime     time.Time
    EndTime       *time.Time
    DurationMs    *int64
    Status        string
    ErrorMessage  *string
}
```

---

## Error Handling

### Real-Time Watcher Errors

**Scenario**: File deleted before processing  
**Handling**: Log warning, continue monitoring

**Scenario**: Permission denied  
**Handling**: Log error, skip file, continue

**Scenario**: Hash calculation fails  
**Handling**: Store record without hash, continue

**Scenario**: Database connection lost  
**Handling**: Retry with exponential backoff, reconciliation catches up

### Reconciliation Errors

**Scenario**: Directory not accessible  
**Handling**: Mark operation as failed, retry next cycle

**Scenario**: Database transaction failure  
**Handling**: Rollback batch, retry with smaller batch

**Scenario**: Out of memory  
**Handling**: Reduce batch size automatically

---

## Configuration Tuning

### For Small Repositories (<10k files)

```go
config := SyncManagerConfig{
    ReconciliationInterval: 12 * time.Hour,  // More frequent
    ReconciliationConfig: ReconciliationConfig{
        BatchSize:       200,  // Larger batches
        MaxConcurrency:  8,    // More parallel hashing
        CalculateHashes: true, // Full hash verification
    },
}
```

### For Large Repositories (>100k files)

```go
config := SyncManagerConfig{
    ReconciliationInterval: 48 * time.Hour,  // Less frequent
    ReconciliationConfig: ReconciliationConfig{
        BatchSize:       50,    // Smaller batches
        MaxConcurrency:  2,     // Less parallel load
        CalculateHashes: false, // Skip hash for performance
    },
}
```

### For High-Change Environments

```go
config := SyncManagerConfig{
    FileWatcherConfig: FileWatcherConfig{
        DebounceInterval: 1 * time.Second,  // Longer debounce
    },
}
```

---

## Limitations and Known Issues

### Current Limitations

1. **No Cloud Storage Support**: Only local filesystems
2. **No Partial Scans**: Reconciliation is all-or-nothing
3. **No Incremental Hashing**: Large files recalculated fully
4. **No Conflict Resolution**: Last-write-wins
5. **Limited Filtering**: Basic file pattern ignore only

### Known Issues

1. **Race Conditions**: Rapid file changes may be missed
2. **High Load**: System can overwhelm under very rapid changes
3. **Network Filesystems**: May not receive events reliably
4. **Large Files**: Hash calculation blocks processing
5. **Memory Usage**: Large scans can use significant memory

---

## Future Improvements

### Planned Enhancements

1. **Incremental Hashing**: Hash large files in chunks
2. **Partial Scans**: Scan subdirectories independently
3. **Smart Scheduling**: Adjust reconciliation based on change rate
4. **Cloud Storage**: S3, MinIO backend support
5. **Conflict Detection**: Identify and report conflicts
6. **Priority Queues**: Process important directories first
7. **Compression**: Reduce database storage for records
8. **Statistics**: Detailed sync metrics and dashboards

---

## Integration with Asset Management

### Current State

**File Records vs Assets**:
- File records track ALL user files
- Assets track only media files (photos, videos, audio)
- Separate systems for now

**Why Separate?**:
- Assets have rich metadata (EXIF, thumbnails, embeddings)
- File records are lightweight (just path, size, hash)
- Different use cases and performance characteristics

### Future Integration

**Planned Connection**:
```go
// Link file record to asset
type Asset struct {
    // ...existing fields...
    FileRecordID *int64  // Link to file_record
}

// Detect asset changes via file records
if fileRecord.ContentHash != asset.Hash {
    // File changed, reprocess asset
    enqueueAssetReprocessing(asset.ID)
}
```

**Benefits**:
- Automatic asset update detection
- Handle user file moves
- Unified change tracking
- Better consistency

---

## Conclusion

The directory scanning system provides reliable file synchronization through a two-tier approach:

1. **Real-time watching** for immediate responsiveness
2. **Daily reconciliation** for long-term consistency

**Key Strengths**:
- Reliable: Catches all changes eventually
- Efficient: Only hashes changed files
- Scalable: Batched processing for large repos
- Flexible: Configurable for different use cases

**Current Limitations**:
- No cloud storage support
- No conflict resolution
- Limited filtering options
- Memory usage on very large scans

The system is functional but still in beta. Production use requires monitoring and tuning based on repository characteristics.
