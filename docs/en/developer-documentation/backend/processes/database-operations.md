# Process Documentation: Database Operations

*Author: Edwin Zhan, documented by AI*

## Overview

Lumilio Photos uses PostgreSQL as its primary database with several modern tools and extensions to provide type-safe queries, vector search capabilities, and job queue management. This document details the database architecture, migration strategy, query patterns, and operational considerations.

## Technology Stack

### Core Database
- **PostgreSQL 15+**: Primary relational database
- **pgxpool**: High-performance connection pooling
- **pgvector**: Vector similarity search extension

### Code Generation and Migration
- **SQLC**: Type-safe Go code generation from SQL
- **Atlas CLI**: Schema migrations
- **River**: Background job queue (separate migration system)

### Key Files
- `server/schema/` - SQL schema definitions
- `server/internal/db/queries/` - SQLC query files
- `server/internal/db/repo/` - Generated SQLC code
- `server/migrations/` - Atlas migration files
- `server/internal/db/migration.go` - Migration logic

---

## Database Schema

### Core Tables

#### assets
**Purpose**: Stores core asset metadata

```sql
CREATE TABLE assets (
    id BIGSERIAL PRIMARY KEY,
    owner_id INTEGER,
    type TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    mime_type TEXT,
    file_size BIGINT NOT NULL,
    hash TEXT,
    width INTEGER,
    height INTEGER,
    duration REAL,
    taken_time TIMESTAMP WITH TIME ZONE NOT NULL,
    specific_metadata JSONB,
    rating INTEGER DEFAULT 0,
    liked BOOLEAN,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_assets_owner ON assets(owner_id);
CREATE INDEX idx_assets_type ON assets(type);
CREATE INDEX idx_assets_hash ON assets(hash);
CREATE INDEX idx_assets_taken_time ON assets(taken_time DESC);
CREATE INDEX idx_assets_created_at ON assets(created_at DESC);
```

**Key Fields**:
- `type`: "image", "video", "audio", "unknown"
- `storage_path`: Relative path within repository
- `hash`: Content hash for deduplication
- `specific_metadata`: JSONB for flexible metadata (EXIF, camera info, etc.)

**Performance Considerations**:
- Indexed on common query patterns
- `taken_time` DESC for chronological browsing
- `hash` for duplicate detection
- JSONB field allows flexible metadata without schema changes

---

#### thumbnails
**Purpose**: Tracks generated thumbnails for assets

```sql
CREATE TABLE thumbnails (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    size_type TEXT NOT NULL, -- '150', '300', '1024'
    storage_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type TEXT DEFAULT 'image/jpeg',
    width INTEGER,
    height INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(asset_id, size_type)
);

CREATE INDEX idx_thumbnails_asset ON thumbnails(asset_id);
```

**Size Types**:
- `150`: Grid view thumbnail
- `300`: List view thumbnail
- `1024`: Lightbox preview

**Cascade Deletion**: Thumbnails deleted when asset deleted

---

#### embeddings
**Purpose**: Stores ML-generated feature vectors for semantic search

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    vector vector(512) NOT NULL,  -- pgvector type
    model_version TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(asset_id)
);

CREATE INDEX idx_embeddings_vector ON embeddings 
USING ivfflat (vector vector_cosine_ops)
WITH (lists = 100);
```

**Vector Indexing**:
- `ivfflat`: Approximate nearest neighbor search
- `vector_cosine_ops`: Cosine similarity
- `lists = 100`: Number of clusters (tune based on data size)

**Performance**:
- Exact search: O(n) - scan all vectors
- Approximate search: O(sqrt(n)) - much faster for large datasets
- Trade-off: Speed vs accuracy

---

#### species_predictions
**Purpose**: Stores ML classification results

```sql
CREATE TABLE species_predictions (
    id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    species_name TEXT NOT NULL,
    confidence REAL NOT NULL,
    model_version TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_species_predictions_asset ON species_predictions(asset_id);
CREATE INDEX idx_species_predictions_species ON species_predictions(species_name);
CREATE INDEX idx_species_predictions_confidence ON species_predictions(confidence DESC);
```

**Usage**:
- Top-K predictions per asset (typically K=3)
- Confidence score 0.0 to 1.0
- Model version for tracking improvements

---

#### albums
**Purpose**: User-created collections of assets

```sql
CREATE TABLE albums (
    id BIGSERIAL PRIMARY KEY,
    owner_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    cover_asset_id BIGINT REFERENCES assets(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_albums_owner ON albums(owner_id);
```

---

#### album_assets
**Purpose**: Many-to-many relationship between albums and assets

```sql
CREATE TABLE album_assets (
    id BIGSERIAL PRIMARY KEY,
    album_id BIGINT NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    asset_id BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    position INTEGER DEFAULT 0,
    added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(album_id, asset_id)
);

CREATE INDEX idx_album_assets_album ON album_assets(album_id, position);
CREATE INDEX idx_album_assets_asset ON album_assets(asset_id);
```

**Position Field**: Allows manual ordering within album

---

#### file_records
**Purpose**: Tracks user-managed files for sync system

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

---

#### sync_operations
**Purpose**: Tracks sync operation history

```sql
CREATE TABLE sync_operations (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL,
    operation_type TEXT NOT NULL,
    files_scanned INTEGER DEFAULT 0,
    files_added INTEGER DEFAULT 0,
    files_updated INTEGER DEFAULT 0,
    files_removed INTEGER DEFAULT 0,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT,
    status TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_sync_ops_repo ON sync_operations(repository_id, start_time DESC);
```

---

### River Queue Tables

**Managed by River migrations, not Atlas**

Key tables:
- `river_job`: Job queue
- `river_leader`: Leader election for workers
- `river_migration`: River's own migration tracking

See [Queue System](../../../../server/internal/queue/README.md) for details.

---

## Migration Strategy

### Two-Track System

#### Track 1: Application Schema (Atlas)
**Purpose**: Main application tables (assets, albums, etc.)  
**Tool**: Atlas CLI  
**Files**: `server/migrations/`

**Migration Flow**:
```go
// server/internal/db/migration.go
func AutoMigrate(ctx context.Context, config DBConfig) error {
    // 1. Check Atlas CLI availability
    if !isAtlasInstalled() {
        return fmt.Errorf("atlas CLI not found")
    }
    
    // 2. Check River CLI availability
    if !isRiverInstalled() {
        return fmt.Errorf("river CLI not found")
    }
    
    // 3. Create migrations directory
    os.MkdirAll("migrations", 0755)
    
    // 4. Generate initial Atlas migration if needed
    if !migrationsExist() {
        generateInitialMigration()
    }
    
    // 5. Apply Atlas migrations
    applyAtlasMigrations(dbURL)
    
    // 6. Run River migrations
    applyRiverMigrations(dbURL)
    
    return nil
}
```

**Manual Commands**:
```bash
# Generate migration
atlas migrate diff migration_name \
  --dir "file://migrations" \
  --to "file://schema" \
  --dev-url "docker://postgres/15/test"

# Apply migrations
atlas migrate apply \
  --dir "file://migrations" \
  --url "$DATABASE_URL"

# Migration status
atlas migrate status \
  --dir "file://migrations" \
  --url "$DATABASE_URL"
```

#### Track 2: River Queue (River CLI)
**Purpose**: Job queue tables  
**Tool**: River CLI  
**Files**: Managed by River internally

**Manual Commands**:
```bash
# Migrate up
river migrate-up \
  --line main \
  --database-url "$DATABASE_URL"

# Migrate down
river migrate-down \
  --line main \
  --database-url "$DATABASE_URL" \
  --max-steps 10
```

### Why Two Systems?

**Atlas for Application**:
- Version-controlled migrations
- Declarative schema definition
- Easy rollback
- Schema diffing

**River for Queue**:
- Tightly integrated with River library
- Automatic updates with River version
- Optimized for queue operations

---

## SQLC Code Generation

### Purpose

Generate type-safe Go code from SQL queries to:
- Eliminate SQL injection risks
- Catch SQL errors at compile time
- Provide ergonomic Go interfaces
- Maintain clear separation of concerns

### Configuration

**File**: `server/sqlc.yaml`

```yaml
version: "2"
sql:
  - schema: "schema"
    queries: "internal/db/queries"
    engine: "postgresql"
    gen:
      go:
        package: "repo"
        out: "internal/db/repo"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_interface: true
        emit_empty_slices: true
```

### Query Files

**Location**: `server/internal/db/queries/*.sql`

**Example**: `assets.sql`

```sql
-- name: GetAsset :one
SELECT * FROM assets
WHERE id = $1;

-- name: ListAssets :many
SELECT * FROM assets
WHERE owner_id = $1
ORDER BY taken_time DESC
LIMIT $2 OFFSET $3;

-- name: CreateAsset :one
INSERT INTO assets (
    owner_id,
    type,
    original_filename,
    storage_path,
    mime_type,
    file_size,
    hash,
    taken_time,
    rating
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateAssetMetadata :exec
UPDATE assets
SET
    width = $2,
    height = $3,
    taken_time = $4,
    specific_metadata = $5,
    updated_at = NOW()
WHERE id = $1;

-- name: DeleteAsset :exec
DELETE FROM assets
WHERE id = $1;

-- name: GetAssetsByHash :many
SELECT * FROM assets
WHERE hash = $1;
```

### Generated Code

**Location**: `server/internal/db/repo/`

**Generated Interface**:
```go
type Querier interface {
    GetAsset(ctx context.Context, id int64) (Asset, error)
    ListAssets(ctx context.Context, arg ListAssetsParams) ([]Asset, error)
    CreateAsset(ctx context.Context, arg CreateAssetParams) (Asset, error)
    UpdateAssetMetadata(ctx context.Context, arg UpdateAssetMetadataParams) error
    DeleteAsset(ctx context.Context, id int64) error
    GetAssetsByHash(ctx context.Context, hash string) ([]Asset, error)
}
```

**Usage in Service**:
```go
type AssetService struct {
    queries *repo.Queries
}

func (s *AssetService) GetAsset(ctx context.Context, id int64) (*repo.Asset, error) {
    asset, err := s.queries.GetAsset(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get asset: %w", err)
    }
    return &asset, nil
}
```

### Regenerating Code

```bash
cd server
sqlc generate
```

**When to Regenerate**:
- After modifying query files
- After schema changes
- After updating SQLC version

---

## Connection Management

### Connection Pooling

**Configuration**: `server/internal/db/db.go`

```go
func New(config DBConfig) (*Database, error) {
    connStr := fmt.Sprintf(
        "postgres://%s:%s@%s:%s/%s?sslmode=%s",
        config.User,
        config.Password,
        config.Host,
        config.Port,
        config.DBName,
        config.SSLMode,
    )
    
    poolConfig, err := pgxpool.ParseConfig(connStr)
    if err != nil {
        return nil, err
    }
    
    // Pool configuration
    poolConfig.MaxConns = 25              // Maximum connections
    poolConfig.MinConns = 5               // Minimum idle connections
    poolConfig.MaxConnLifetime = time.Hour    // Recycle after 1 hour
    poolConfig.MaxConnIdleTime = 30 * time.Minute
    poolConfig.HealthCheckPeriod = 1 * time.Minute
    
    pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
    if err != nil {
        return nil, err
    }
    
    return &Database{
        Pool:    pool,
        Queries: repo.New(pool),
    }, nil
}
```

**Pool Sizing Guidelines**:

**For Development**:
- MaxConns: 10-25
- MinConns: 2-5

**For Production**:
- MaxConns: 50-100 (depends on workload)
- MinConns: 10-20
- Formula: `MaxConns = ((CPU cores * 2) + effective_spindle_count)`

### Health Checks

```go
func (db *Database) Ping(ctx context.Context) error {
    return db.Pool.Ping(ctx)
}

func (db *Database) Stats() *pgxpool.Stat {
    return db.Pool.Stat()
}
```

**Monitored Metrics**:
- `AcquireCount`: Total connections acquired
- `AcquireDuration`: Time waiting for connection
- `AcquiredConns`: Currently acquired connections
- `IdleConns`: Idle connections in pool
- `TotalConns`: Total connections (idle + acquired)

---

## Query Patterns and Best Practices

### Pattern 1: Simple CRUD

**Get Single Record**:
```sql
-- name: GetAsset :one
SELECT * FROM assets WHERE id = $1;
```

**List with Pagination**:
```sql
-- name: ListAssets :many
SELECT * FROM assets
WHERE owner_id = $1
ORDER BY taken_time DESC
LIMIT $2 OFFSET $3;
```

**Create**:
```sql
-- name: CreateAsset :one
INSERT INTO assets (...) VALUES (...)
RETURNING *;
```

**Update**:
```sql
-- name: UpdateAsset :exec
UPDATE assets SET ... WHERE id = $1;
```

**Delete**:
```sql
-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = $1;
```

---

### Pattern 2: Joins

**Asset with Thumbnails**:
```sql
-- name: GetAssetWithThumbnails :many
SELECT 
    a.*,
    t.id as thumbnail_id,
    t.size_type,
    t.storage_path as thumbnail_path
FROM assets a
LEFT JOIN thumbnails t ON t.asset_id = a.id
WHERE a.id = $1;
```

**Album with Asset Count**:
```sql
-- name: ListAlbumsWithCount :many
SELECT 
    a.*,
    COUNT(aa.asset_id) as asset_count
FROM albums a
LEFT JOIN album_assets aa ON aa.album_id = a.id
WHERE a.owner_id = $1
GROUP BY a.id
ORDER BY a.updated_at DESC;
```

---

### Pattern 3: Batch Operations

**Batch Insert Assets**:
```sql
-- name: BatchCreateAssets :copyfrom
INSERT INTO assets (
    owner_id,
    type,
    original_filename,
    storage_path,
    file_size,
    hash,
    taken_time
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);
```

**Usage**:
```go
// Much faster than individual inserts
rows := []repo.BatchCreateAssetsParams{
    {OwnerID: 1, Type: "image", ...},
    {OwnerID: 1, Type: "image", ...},
    // ... many more
}

count, err := queries.BatchCreateAssets(ctx, rows)
```

**Performance**: ~10-100x faster than individual inserts

---

### Pattern 4: Vector Search

**Nearest Neighbors**:
```sql
-- name: FindSimilarAssets :many
SELECT 
    a.*,
    1 - (e1.vector <=> e2.vector) as similarity
FROM embeddings e1
CROSS JOIN embeddings e2
JOIN assets a ON a.id = e2.asset_id
WHERE e1.asset_id = $1
  AND e2.asset_id != $1
ORDER BY e1.vector <=> e2.vector
LIMIT $2;
```

**Operators**:
- `<=>`: Cosine distance (0 = identical, 2 = opposite)
- `<->`: L2 distance (Euclidean)
- `<#>`: Inner product

**Conversion**:
- Cosine similarity = 1 - cosine distance
- Higher similarity = more similar

---

### Pattern 5: JSONB Queries

**Query Metadata**:
```sql
-- name: GetAssetsByCamera :many
SELECT * FROM assets
WHERE specific_metadata->>'Camera' = $1
ORDER BY taken_time DESC;

-- name: GetAssetsWithGPS :many
SELECT * FROM assets
WHERE specific_metadata ? 'GPSLatitude'
  AND specific_metadata ? 'GPSLongitude'
LIMIT $1 OFFSET $2;
```

**JSONB Operators**:
- `->`: Get JSON object field
- `->>`: Get JSON object field as text
- `?`: Does key exist?
- `@>`: Contains JSON?
- `<@`: Is contained by JSON?

---

## Transactions

### Current State: No Transaction Wrappers

**Issue**: Operations not atomic
```go
// NOT ATOMIC - if step 2 fails, step 1 persists
asset, err := service.CreateAsset(ctx, params)
if err != nil {
    return err
}

err = service.CreateThumbnail(ctx, asset.ID, thumbnailParams)
if err != nil {
    // asset exists but no thumbnail!
    return err
}
```

### Future: Transaction Support

**Pattern**:
```go
func (s *AssetService) CreateAssetWithThumbnails(
    ctx context.Context,
    assetParams CreateAssetParams,
    thumbnailParams []CreateThumbnailParams,
) (*Asset, error) {
    var asset *Asset
    
    err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
        queries := s.queries.WithTx(tx)
        
        // Step 1: Create asset
        a, err := queries.CreateAsset(ctx, assetParams)
        if err != nil {
            return err
        }
        asset = &a
        
        // Step 2: Create thumbnails
        for _, params := range thumbnailParams {
            params.AssetID = asset.ID
            _, err := queries.CreateThumbnail(ctx, params)
            if err != nil {
                return err  // Rollback
            }
        }
        
        return nil  // Commit
    })
    
    return asset, err
}
```

**Benefits**:
- Atomic operations
- Consistent state
- Automatic rollback on error

---

## Performance Optimization

### Indexing Strategy

**High-Priority Indexes** (already implemented):
- Foreign keys (automatic in queries)
- Sort columns (`taken_time DESC`, `created_at DESC`)
- Filter columns (`owner_id`, `type`, `hash`)
- Vector similarity (`embeddings.vector`)

**Future Indexes** (as needed):
- Composite indexes for common queries
- Partial indexes for specific conditions
- Full-text search indexes

### Query Optimization

**Use EXPLAIN ANALYZE**:
```sql
EXPLAIN ANALYZE
SELECT * FROM assets
WHERE owner_id = 1
ORDER BY taken_time DESC
LIMIT 20;
```

**Look for**:
- Seq Scan → Add index
- High cost → Optimize query
- Slow actual time → Database tuning needed

### Connection Pool Tuning

**Monitor**:
```go
stats := db.Pool.Stat()
log.Printf("Connections: %d/%d (idle: %d)", 
    stats.AcquiredConns(),
    stats.TotalConns(),
    stats.IdleConns())
```

**Tune based on**:
- Average wait time
- Connection saturation
- Database CPU/memory

---

## Backup and Recovery

### Manual Backup

```bash
pg_dump -h localhost -U postgres -d lumiliophotos > backup.sql
```

### Point-in-Time Recovery

**Enable WAL archiving**:
```sql
ALTER SYSTEM SET wal_level = replica;
ALTER SYSTEM SET archive_mode = on;
ALTER SYSTEM SET archive_command = 'cp %p /path/to/archive/%f';
```

### Future: Automated Backups

- Daily full backups
- Continuous WAL archiving
- S3 storage for backups
- Automated restore testing

---

## Monitoring and Observability

### Key Metrics

**Query Performance**:
- Slow query log (queries > 1s)
- Query count by type
- Average query duration

**Connection Pool**:
- Pool utilization %
- Wait time for connections
- Connection errors

**Database Health**:
- CPU usage
- Memory usage
- Disk I/O
- Cache hit ratio

### Future: Prometheus Integration

```go
// Expose metrics
http.Handle("/metrics", promhttp.Handler())

// Track query duration
queryDuration := prometheus.NewHistogramVec(...)
queryDuration.WithLabelValues("GetAsset").Observe(duration.Seconds())
```

---

## Common Operations

### Add New Table

1. Update schema: `server/schema/schema.sql`
2. Create query file: `server/internal/db/queries/new_table.sql`
3. Generate migration: `atlas migrate diff add_new_table ...`
4. Generate SQLC code: `sqlc generate`
5. Apply migration: `atlas migrate apply ...`

### Add New Query

1. Edit query file: `server/internal/db/queries/*.sql`
2. Regenerate: `sqlc generate`
3. Use in service layer

### Add Index

1. Update schema with index
2. Generate migration
3. Apply during low-traffic period
4. Monitor query performance

---

## Troubleshooting

### Connection Pool Exhausted

**Symptoms**: Timeouts acquiring connections  
**Causes**: Too many slow queries, too few connections  
**Solutions**: Increase MaxConns, optimize queries, add connection limits

### Slow Queries

**Symptoms**: High query duration  
**Causes**: Missing indexes, inefficient queries  
**Solutions**: Add indexes, rewrite queries, use EXPLAIN ANALYZE

### Migration Failures

**Symptoms**: Migration doesn't apply  
**Causes**: Schema conflicts, data constraints  
**Solutions**: Fix schema, migrate data manually, rollback if needed

---

## Future Enhancements

1. **Read Replicas**: Scale read operations
2. **Sharding**: Distribute data across databases
3. **Caching Layer**: Redis for frequently accessed data
4. **Query Logging**: Track all queries for analysis
5. **Automated Performance Tuning**: Suggest index improvements
6. **Multi-Tenant Isolation**: Row-level security

---

## Conclusion

The database layer provides:
- Type-safe queries via SQLC
- Robust migrations via Atlas + River
- High performance via pgxpool
- Vector search via pgvector
- Background jobs via River tables

**Strengths**:
- Type safety prevents SQL injection
- Connection pooling handles concurrency
- Migrations are version-controlled
- Modern PostgreSQL features utilized

**Areas for Improvement**:
- Transaction wrappers needed
- Monitoring integration
- Automated backup system
- Performance profiling tools
