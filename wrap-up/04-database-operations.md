# Database Operations - Detailed Analysis

## Overview

The database layer provides persistent storage for all application data, from asset metadata to user information. It uses PostgreSQL with specialized extensions and follows a layered architecture: raw SQL migrations → SQLC-generated queries → service layer → application code.

## Architecture

### Database Stack

```
┌──────────────────────────────────────────┐
│        Application Code                  │
│  (Handlers, Processors, Workers)         │
└──────────────┬───────────────────────────┘
               │
┌──────────────▼───────────────────────────┐
│         Service Layer                    │
│  (AssetService, AlbumService, etc.)      │
└──────────────┬───────────────────────────┘
               │
┌──────────────▼───────────────────────────┐
│      SQLC Generated Queries              │
│    (Type-safe, compile-time checked)     │
└──────────────┬───────────────────────────┘
               │
┌──────────────▼───────────────────────────┐
│      PostgreSQL Database                 │
│  + pgvector (embeddings)                 │
│  + River (job queue tables)              │
└──────────────────────────────────────────┘
```

### Technology Choices

**PostgreSQL**: Chosen for:
- ACID guarantees (reliability)
- JSON/JSONB support (flexible metadata)
- Full-text search (asset search)
- Vector similarity (pgvector for CLIP)
- Proven scalability

**SQLC**: Type-safe query generation:
- Compile-time SQL validation
- Generated Go structs and functions
- No runtime reflection
- Easy to review (generated code is readable)

**pgvector**: Vector similarity search:
- Native PostgreSQL extension
- Efficient nearest-neighbor search
- HNSW and IVFFlat index support
- Perfect for CLIP embeddings

## Database Schema

### Core Tables

#### users
Stores user accounts and authentication:

```sql
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    avatar_url TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_login TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
```

#### repositories
Stores repository information:

```sql
CREATE TABLE repositories (
    repo_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    path TEXT NOT NULL UNIQUE,
    owner_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    storage_strategy VARCHAR(20) DEFAULT 'date',  -- 'date', 'flat', 'cas'
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_repositories_owner ON repositories(owner_id);
CREATE INDEX idx_repositories_default ON repositories(is_default);
```

#### assets
Core asset information:

```sql
CREATE TABLE assets (
    asset_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_filename VARCHAR(255) NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_hash TEXT,
    mime_type VARCHAR(100),
    asset_type VARCHAR(20) NOT NULL,  -- 'photo', 'video', 'audio'
    
    -- Repository relationship
    repository_id UUID REFERENCES repositories(repo_id) ON DELETE CASCADE,
    
    -- Ownership
    owner_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    
    -- Timestamps
    uploaded_at TIMESTAMP NOT NULL,
    taken_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    -- Photo metadata (nullable for non-photos)
    camera_make VARCHAR(100),
    camera_model VARCHAR(100),
    lens_model VARCHAR(100),
    iso INTEGER,
    aperture NUMERIC(5,2),
    shutter_speed VARCHAR(50),
    focal_length NUMERIC(6,2),
    
    -- Location
    latitude NUMERIC(10,8),
    longitude NUMERIC(11,8),
    altitude NUMERIC(10,2),
    
    -- Dimensions (for photos/videos)
    width INTEGER,
    height INTEGER,
    duration NUMERIC(10,2),  -- Video/audio duration in seconds
    
    -- User metadata
    description TEXT,
    rating INTEGER CHECK (rating >= 0 AND rating <= 5),
    liked BOOLEAN DEFAULT false,
    
    -- Processing flags
    is_raw BOOLEAN DEFAULT false,
    is_transcoded BOOLEAN DEFAULT false,
    transcoded_path TEXT
);

-- Indexes for common queries
CREATE INDEX idx_assets_type ON assets(asset_type);
CREATE INDEX idx_assets_repository ON assets(repository_id);
CREATE INDEX idx_assets_owner ON assets(owner_id);
CREATE INDEX idx_assets_hash ON assets(file_hash);
CREATE INDEX idx_assets_uploaded ON assets(uploaded_at DESC);
CREATE INDEX idx_assets_taken ON assets(taken_at DESC);
CREATE INDEX idx_assets_rating ON assets(rating DESC);
CREATE INDEX idx_assets_liked ON assets(liked) WHERE liked = true;
CREATE INDEX idx_assets_camera ON assets(camera_make, camera_model);
```

#### thumbnails
Generated thumbnail information:

```sql
CREATE TABLE thumbnails (
    thumbnail_id SERIAL PRIMARY KEY,
    asset_id UUID REFERENCES assets(asset_id) ON DELETE CASCADE,
    size VARCHAR(20) NOT NULL,  -- 'small', 'medium', 'large'
    file_path TEXT NOT NULL,
    file_size BIGINT,
    width INTEGER,
    height INTEGER,
    mime_type VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(asset_id, size)
);

CREATE INDEX idx_thumbnails_asset ON thumbnails(asset_id);
```

#### embeddings
CLIP embeddings for semantic search:

```sql
CREATE TABLE embeddings (
    embedding_id SERIAL PRIMARY KEY,
    asset_id UUID REFERENCES assets(asset_id) ON DELETE CASCADE,
    embedding vector(512),  -- pgvector type
    model_version VARCHAR(50) DEFAULT 'clip-vit-base',
    created_at TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(asset_id)
);

-- HNSW index for fast similarity search
CREATE INDEX idx_embeddings_vector ON embeddings 
USING hnsw (embedding vector_cosine_ops) 
WITH (m = 16, ef_construction = 64);
```

#### tags
Tagging system for categorization:

```sql
CREATE TABLE tags (
    tag_id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    category VARCHAR(50),  -- 'species', 'object', 'scene', 'custom'
    is_ai_generated BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_tags_category ON tags(category);
CREATE INDEX idx_tags_name ON tags(name);
```

#### asset_tags
Many-to-many relationship:

```sql
CREATE TABLE asset_tags (
    asset_id UUID REFERENCES assets(asset_id) ON DELETE CASCADE,
    tag_id INTEGER REFERENCES tags(tag_id) ON DELETE CASCADE,
    confidence NUMERIC(5,4),  -- AI confidence score (0-1)
    source VARCHAR(50),  -- 'user', 'clip', 'smart_classify'
    created_at TIMESTAMP DEFAULT NOW(),
    
    PRIMARY KEY (asset_id, tag_id)
);

CREATE INDEX idx_asset_tags_asset ON asset_tags(asset_id);
CREATE INDEX idx_asset_tags_tag ON asset_tags(tag_id);
CREATE INDEX idx_asset_tags_confidence ON asset_tags(confidence DESC);
```

#### albums
Photo album organization:

```sql
CREATE TABLE albums (
    album_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    cover_asset_id UUID REFERENCES assets(asset_id) ON DELETE SET NULL,
    owner_id INTEGER REFERENCES users(user_id) ON DELETE CASCADE,
    is_public BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_albums_owner ON albums(owner_id);
```

#### album_assets
Many-to-many relationship:

```sql
CREATE TABLE album_assets (
    album_id INTEGER REFERENCES albums(album_id) ON DELETE CASCADE,
    asset_id UUID REFERENCES assets(asset_id) ON DELETE CASCADE,
    position INTEGER,  -- Order within album
    added_at TIMESTAMP DEFAULT NOW(),
    
    PRIMARY KEY (album_id, asset_id)
);

CREATE INDEX idx_album_assets_album ON album_assets(album_id, position);
CREATE INDEX idx_album_assets_asset ON album_assets(asset_id);
```

#### species_predictions
AI-powered species classification:

```sql
CREATE TABLE species_predictions (
    prediction_id SERIAL PRIMARY KEY,
    asset_id UUID REFERENCES assets(asset_id) ON DELETE CASCADE,
    species_name VARCHAR(255) NOT NULL,
    common_name VARCHAR(255),
    confidence NUMERIC(5,4) NOT NULL,
    model_version VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_species_predictions_asset ON species_predictions(asset_id);
CREATE INDEX idx_species_predictions_confidence ON species_predictions(confidence DESC);
```

### File Synchronization Tables

#### file_records
Tracks files in user-managed areas:

```sql
CREATE TABLE file_records (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    mod_time TIMESTAMP NOT NULL,
    content_hash TEXT,
    last_scanned TIMESTAMP NOT NULL,
    scan_generation INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    CONSTRAINT file_records_unique UNIQUE (repository_id, file_path)
);

CREATE INDEX idx_file_records_repo ON file_records(repository_id);
CREATE INDEX idx_file_records_hash ON file_records(content_hash);
CREATE INDEX idx_file_records_scanned ON file_records(last_scanned);
```

#### sync_operations
Tracks sync operations:

```sql
CREATE TABLE sync_operations (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL,  -- 'realtime', 'reconciliation', 'startup'
    files_scanned INTEGER DEFAULT 0,
    files_added INTEGER DEFAULT 0,
    files_updated INTEGER DEFAULT 0,
    files_removed INTEGER DEFAULT 0,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration_ms BIGINT,
    status TEXT NOT NULL,  -- 'running', 'completed', 'failed'
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_sync_ops_repo ON sync_operations(repository_id);
CREATE INDEX idx_sync_ops_time ON sync_operations(start_time DESC);
```

### Queue Tables (River)

River queue system adds its own tables:
- `river_job`: Queued jobs
- `river_leader`: Leader election for distributed processing
- `river_migration`: Migration tracking
- `river_queue`: Queue configuration

These are managed by River CLI migrations.

## SQLC - Type-Safe Queries

### Configuration

**Location**: `server/sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/db/repo/queries/"
    schema: "schema/"
    gen:
      go:
        package: "repo"
        out: "internal/db/repo"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_exact_table_names: false
```

### Query Definition

**Location**: `server/internal/db/repo/queries/assets.sql`

Example query:

```sql
-- name: GetAsset :one
SELECT * FROM assets
WHERE asset_id = $1;

-- name: ListAssets :many
SELECT * FROM assets
ORDER BY uploaded_at DESC
LIMIT $1 OFFSET $2;

-- name: CreateAsset :one
INSERT INTO assets (
    original_filename,
    file_path,
    file_size,
    file_hash,
    mime_type,
    asset_type,
    repository_id,
    owner_id,
    uploaded_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateAssetMetadata :exec
UPDATE assets
SET 
    taken_at = COALESCE(sqlc.narg('taken_at'), taken_at),
    camera_make = COALESCE(sqlc.narg('camera_make'), camera_make),
    camera_model = COALESCE(sqlc.narg('camera_model'), camera_model),
    latitude = COALESCE(sqlc.narg('latitude'), latitude),
    longitude = COALESCE(sqlc.narg('longitude'), longitude),
    updated_at = NOW()
WHERE asset_id = sqlc.arg('asset_id');

-- name: GetAssetsByHash :many
SELECT * FROM assets
WHERE file_hash = $1;

-- name: SearchAssetsByFilename :many
SELECT * FROM assets
WHERE original_filename ILIKE '%' || $1 || '%'
ORDER BY uploaded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAssetsByOwnerAndType :many
SELECT * FROM assets
WHERE owner_id = $1 
  AND asset_type = $2
ORDER BY uploaded_at DESC
LIMIT $3 OFFSET $4;
```

### Generated Code

SQLC generates type-safe Go code:

```go
// Generated: server/internal/db/repo/assets.sql.go

type Asset struct {
    AssetID           pgtype.UUID   `json:"asset_id"`
    OriginalFilename  string        `json:"original_filename"`
    FilePath          string        `json:"file_path"`
    FileSize          int64         `json:"file_size"`
    FileHash          *string       `json:"file_hash"`
    MimeType          *string       `json:"mime_type"`
    AssetType         string        `json:"asset_type"`
    RepositoryID      pgtype.UUID   `json:"repository_id"`
    OwnerID           *int32        `json:"owner_id"`
    UploadedAt        time.Time     `json:"uploaded_at"`
    TakenAt           *time.Time    `json:"taken_at"`
    // ... other fields
}

type CreateAssetParams struct {
    OriginalFilename string      `json:"original_filename"`
    FilePath         string      `json:"file_path"`
    FileSize         int64       `json:"file_size"`
    FileHash         *string     `json:"file_hash"`
    MimeType         *string     `json:"mime_type"`
    AssetType        string      `json:"asset_type"`
    RepositoryID     pgtype.UUID `json:"repository_id"`
    OwnerID          *int32      `json:"owner_id"`
    UploadedAt       time.Time   `json:"uploaded_at"`
}

func (q *Queries) GetAsset(ctx context.Context, assetID pgtype.UUID) (Asset, error) {
    row := q.db.QueryRow(ctx, getAsset, assetID)
    var i Asset
    err := row.Scan(
        &i.AssetID,
        &i.OriginalFilename,
        &i.FilePath,
        // ... other fields
    )
    return i, err
}

func (q *Queries) CreateAsset(ctx context.Context, arg CreateAssetParams) (Asset, error) {
    row := q.db.QueryRow(ctx, createAsset,
        arg.OriginalFilename,
        arg.FilePath,
        arg.FileSize,
        // ... other args
    )
    var i Asset
    err := row.Scan(/* ... */)
    return i, err
}
```

**Benefits**:
- Compile-time SQL validation
- Type-safe parameters
- Automatic struct mapping
- No SQL injection vulnerabilities
- IDE autocomplete support

### Running SQLC

```bash
# Generate code from SQL queries
sqlc generate

# Verify SQL syntax
sqlc verify

# Show version
sqlc version
```

## Service Layer

### AssetService

**Location**: `server/internal/service/asset_service.go`

The service layer provides business logic on top of SQLC queries:

```go
type AssetService interface {
    // Basic CRUD
    GetAsset(ctx context.Context, id uuid.UUID) (*repo.Asset, error)
    CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error)
    UpdateAssetMetadata(ctx context.Context, id uuid.UUID, metadata []byte) error
    DeleteAsset(ctx context.Context, id uuid.UUID) error
    
    // Queries
    GetAssetsByType(ctx context.Context, assetType string, limit, offset int) ([]repo.Asset, error)
    GetAssetsByOwner(ctx context.Context, ownerID int, limit, offset int) ([]repo.Asset, error)
    SearchAssets(ctx context.Context, query string, assetType *string, useVector bool, limit, offset int) ([]repo.Asset, error)
    
    // Filtering
    FilterAssets(ctx context.Context, filters AssetFilters) ([]repo.Asset, error)
    
    // Relationships
    AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
    AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
    
    // ML Features
    SaveNewEmbedding(ctx context.Context, assetID pgtype.UUID, embedding []float32) error
    SaveNewSpeciesPredictions(ctx context.Context, assetID pgtype.UUID, predictions []SpeciesPrediction) error
    
    // Thumbnails
    CreateThumbnail(ctx context.Context, assetID uuid.UUID, size string, thumbnailPath string) (*repo.Thumbnail, error)
    GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*repo.Thumbnail, error)
}
```

### Implementation Examples

#### Create Asset

```go
func (s *assetService) CreateAssetRecord(ctx context.Context, params repo.CreateAssetParams) (*repo.Asset, error) {
    // Validate asset type
    if params.AssetType != "photo" && params.AssetType != "video" && params.AssetType != "audio" {
        return nil, ErrInvalidAssetType
    }
    
    // Check for duplicates
    if params.FileHash != nil && *params.FileHash != "" {
        existing, err := s.queries.GetAssetsByHash(ctx, *params.FileHash)
        if err == nil && len(existing) > 0 {
            // Duplicate found - return existing
            return &existing[0], nil
        }
    }
    
    // Create new asset
    asset, err := s.queries.CreateAsset(ctx, params)
    if err != nil {
        return nil, fmt.Errorf("failed to create asset: %w", err)
    }
    
    return &asset, nil
}
```

#### Save CLIP Embedding

```go
func (s *assetService) SaveNewEmbedding(ctx context.Context, assetID pgtype.UUID, embedding []float32) error {
    // Convert to pgvector format
    vector := pgvector_go.NewVector(embedding)
    
    // Upsert embedding (replace if exists)
    err := s.queries.UpsertEmbedding(ctx, repo.UpsertEmbeddingParams{
        AssetID:      assetID,
        Embedding:    vector,
        ModelVersion: "clip-vit-base",
    })
    
    if err != nil {
        return fmt.Errorf("failed to save embedding: %w", err)
    }
    
    return nil
}
```

#### Vector Search

```go
func (s *assetService) SearchAssetsVector(ctx context.Context, query string, limit int) ([]repo.Asset, error) {
    // Get query embedding from ML service
    queryEmbedding, err := s.ml.GetTextEmbedding(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // Convert to pgvector
    queryVector := pgvector_go.NewVector(queryEmbedding)
    
    // Find similar embeddings
    embeddings, err := s.queries.SearchEmbeddingsBySimilarity(ctx, repo.SearchEmbeddingsBySimilarityParams{
        Embedding: queryVector,
        Limit:     int32(limit),
    })
    
    if err != nil {
        return nil, err
    }
    
    // Get assets for embeddings
    var assets []repo.Asset
    for _, emb := range embeddings {
        asset, err := s.queries.GetAsset(ctx, emb.AssetID)
        if err == nil {
            assets = append(assets, asset)
        }
    }
    
    return assets, nil
}
```

### AlbumService

**Location**: `server/internal/service/album_service.go`

```go
type AlbumService interface {
    CreateAlbum(ctx context.Context, name, description string, ownerID int) (*repo.Album, error)
    GetAlbum(ctx context.Context, albumID int) (*repo.Album, error)
    ListAlbums(ctx context.Context, ownerID int) ([]repo.Album, error)
    UpdateAlbum(ctx context.Context, albumID int, name, description *string) error
    DeleteAlbum(ctx context.Context, albumID int) error
    
    AddAssetToAlbum(ctx context.Context, albumID int, assetID uuid.UUID, position int) error
    RemoveAssetFromAlbum(ctx context.Context, albumID int, assetID uuid.UUID) error
    GetAlbumAssets(ctx context.Context, albumID int) ([]repo.Asset, error)
}
```

### AuthService

**Location**: `server/internal/service/auth_service.go`

```go
type AuthService interface {
    Register(ctx context.Context, username, email, password string) (*repo.User, error)
    Login(ctx context.Context, username, password string) (*repo.User, string, error)
    ValidateToken(token string) (*TokenClaims, error)
    RefreshToken(ctx context.Context, refreshToken string) (string, error)
    GetUserByID(ctx context.Context, userID int) (*repo.User, error)
}
```

## Database Migrations

### Migration Strategy

Two-tier migration system:
1. **Atlas**: Application schema migrations
2. **River**: Queue system migrations

### Atlas Migrations

**Location**: `server/migrations/`

Migration files follow naming convention:
- `000_extensions.sql`: Enable PostgreSQL extensions
- `001_users.sql`: User tables
- `002_assets.sql`: Asset tables
- `003_tags_albums.sql`: Relationship tables
- `004_repo.sql`: Repository tables
- `005_file_sync.sql`: Sync tables

Example migration:

```sql
-- 002_assets.sql
CREATE TABLE assets (
    asset_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_filename VARCHAR(255) NOT NULL,
    -- ... other columns
);

CREATE INDEX idx_assets_type ON assets(asset_type);
CREATE INDEX idx_assets_uploaded ON assets(uploaded_at DESC);
```

### Running Migrations

```bash
# Set database URL
export DATABASE_URL="postgresql://user:pass@localhost:5432/lumilio"

# Run Atlas migrations (application schema)
atlas migrate apply \
  --dir file://server/migrations \
  --url "$DATABASE_URL"

# Run River migrations (queue tables)
river migrate-up \
  --database-url "$DATABASE_URL"
```

### Automatic Migration

Migrations run automatically on server startup:

**Location**: `server/cmd/main.go`

```go
func main() {
    // Load config
    dbConfig := config.LoadDBConfig()
    
    // Run migrations
    if err := db.AutoMigrate(ctx, dbConfig); err != nil {
        log.Printf("Warning: Failed to run migrations: %v", err)
        log.Println("Run manually: atlas migrate apply ...")
    }
    
    // ... continue startup
}
```

## Transaction Management

### Basic Transactions

```go
func (s *assetService) CreateAssetWithTags(ctx context.Context, asset CreateAssetParams, tagIDs []int) error {
    // Begin transaction
    tx, err := s.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)
    
    // Create queries with transaction
    qtx := s.queries.WithTx(tx)
    
    // Create asset
    newAsset, err := qtx.CreateAsset(ctx, asset)
    if err != nil {
        return err
    }
    
    // Add tags
    for _, tagID := range tagIDs {
        err = qtx.AddTagToAsset(ctx, repo.AddTagToAssetParams{
            AssetID: newAsset.AssetID,
            TagID:   int32(tagID),
        })
        if err != nil {
            return err
        }
    }
    
    // Commit
    return tx.Commit(ctx)
}
```

### River Queue Integration

River uses transactions for job insertion:

```go
func (h *AssetHandler) UploadAsset(c *gin.Context) {
    // ... upload logic
    
    // Insert job in transaction
    job, err := h.QueueClient.Insert(ctx, 
        queue.ProcessAssetArgs{...},
        &river.InsertOpts{Queue: "process_asset"},
    )
    
    // Job is only visible after transaction commits
}
```

## Performance Optimization

### Indexing Strategy

```sql
-- Covering index for common query
CREATE INDEX idx_assets_owner_type_uploaded 
ON assets(owner_id, asset_type, uploaded_at DESC);

-- Partial index for liked assets
CREATE INDEX idx_assets_liked 
ON assets(liked) 
WHERE liked = true;

-- GIN index for full-text search
CREATE INDEX idx_assets_filename_gin 
ON assets 
USING gin(to_tsvector('english', original_filename));

-- HNSW index for vector similarity
CREATE INDEX idx_embeddings_vector 
ON embeddings 
USING hnsw (embedding vector_cosine_ops);
```

### Query Optimization

```go
// Bad: N+1 query problem
func GetAssetsWithThumbnails(ctx context.Context, limit int) ([]AssetWithThumbnails, error) {
    assets, _ := queries.ListAssets(ctx, limit)
    
    for i, asset := range assets {
        thumbnails, _ := queries.GetThumbnailsByAssetID(ctx, asset.AssetID)
        assets[i].Thumbnails = thumbnails  // N additional queries!
    }
    
    return assets, nil
}

// Good: Join query
func GetAssetsWithThumbnails(ctx context.Context, limit int) ([]AssetWithThumbnails, error) {
    // Single query with JOIN
    return queries.ListAssetsWithThumbnails(ctx, limit)
}
```

SQL query:

```sql
-- name: ListAssetsWithThumbnails :many
SELECT 
    a.*,
    jsonb_agg(
        jsonb_build_object(
            'thumbnail_id', t.thumbnail_id,
            'size', t.size,
            'file_path', t.file_path
        )
    ) as thumbnails
FROM assets a
LEFT JOIN thumbnails t ON a.asset_id = t.asset_id
GROUP BY a.asset_id
ORDER BY a.uploaded_at DESC
LIMIT $1;
```

### Connection Pooling

```go
// Configure connection pool
poolConfig, err := pgxpool.ParseConfig(dbURL)
poolConfig.MaxConns = 20
poolConfig.MinConns = 5
poolConfig.MaxConnLifetime = time.Hour
poolConfig.MaxConnIdleTime = 30 * time.Minute

pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
```

### Prepared Statements

SQLC generates prepared statements automatically:

```go
// Generated code uses prepared statements
const getAsset = `SELECT * FROM assets WHERE asset_id = $1`

func (q *Queries) GetAsset(ctx context.Context, assetID pgtype.UUID) (Asset, error) {
    // Uses prepared statement (parsed once, reused many times)
    row := q.db.QueryRow(ctx, getAsset, assetID)
    // ...
}
```

## Monitoring and Maintenance

### Database Metrics

```sql
-- Active connections
SELECT count(*) FROM pg_stat_activity;

-- Slow queries
SELECT query, mean_exec_time, calls 
FROM pg_stat_statements 
ORDER BY mean_exec_time DESC 
LIMIT 10;

-- Index usage
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE idx_scan = 0  -- Unused indexes
ORDER BY tablename;

-- Table sizes
SELECT 
    tablename,
    pg_size_pretty(pg_total_relation_size(tablename::text)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(tablename::text) DESC;
```

### Vacuum and Analyze

```sql
-- Automatic vacuum (runs automatically)
-- Manual vacuum for immediate cleanup
VACUUM ANALYZE assets;

-- Full vacuum (locks table, rarely needed)
VACUUM FULL assets;
```

### Backup and Recovery

```bash
# Full backup
pg_dump -Fc lumilio > backup.dump

# Restore
pg_restore -d lumilio backup.dump

# Backup specific tables
pg_dump -t assets -t thumbnails lumilio > assets_backup.sql
```

## Testing

### Unit Tests with Test Database

```go
func setupTestDB(t *testing.T) *pgxpool.Pool {
    ctx := context.Background()
    
    // Connect to test database
    pool, err := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
    require.NoError(t, err)
    
    // Run migrations
    err = runMigrations(ctx, pool)
    require.NoError(t, err)
    
    // Clean up after test
    t.Cleanup(func() {
        pool.Close()
    })
    
    return pool
}

func TestAssetService_CreateAsset(t *testing.T) {
    pool := setupTestDB(t)
    queries := repo.New(pool)
    service := NewAssetService(queries, nil, nil)
    
    ctx := context.Background()
    
    // Create asset
    asset, err := service.CreateAssetRecord(ctx, repo.CreateAssetParams{
        OriginalFilename: "test.jpg",
        FilePath:         "inbox/test.jpg",
        FileSize:         1024,
        AssetType:        "photo",
        UploadedAt:       time.Now(),
    })
    
    require.NoError(t, err)
    assert.NotNil(t, asset)
    assert.Equal(t, "test.jpg", asset.OriginalFilename)
}
```

## Security Considerations

### SQL Injection Prevention

SQLC-generated queries use parameterized queries:

```go
// Safe: Uses $1 placeholder
queries.GetAsset(ctx, assetID)

// NEVER do this (SQL injection risk):
db.Exec("SELECT * FROM assets WHERE asset_id = '" + assetID + "'")
```

### Row-Level Security

```sql
-- Enable RLS
ALTER TABLE assets ENABLE ROW LEVEL SECURITY;

-- Policy: Users can only see their own assets
CREATE POLICY asset_owner_policy ON assets
FOR SELECT
USING (owner_id = current_user_id());
```

### Sensitive Data

```sql
-- Don't store plaintext passwords
-- Use bcrypt hash
INSERT INTO users (username, password_hash)
VALUES ('john', crypt('password', gen_salt('bf')));

-- Verify password
SELECT * FROM users 
WHERE username = 'john' 
  AND password_hash = crypt('password', password_hash);
```

## Future Improvements

1. **Read Replicas**: Scale read-heavy queries
2. **Partitioning**: Partition large tables by date/repository
3. **Materialized Views**: Pre-compute expensive aggregations
4. **Query Cache**: Cache frequent queries in Redis
5. **Sharding**: Distribute data across multiple databases
6. **Audit Logs**: Track all data changes
7. **Soft Deletes**: Add `deleted_at` column instead of hard delete

## Related Documentation

- [SQLC Documentation](https://docs.sqlc.dev/)
- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [River Queue Documentation](https://riverqueue.com/docs)
- [Database README](../server/internal/db/README.md)

---

*This document is part of the Lumilio Photos server wrap-up documentation.*
