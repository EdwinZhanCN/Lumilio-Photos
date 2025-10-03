# Lumilio Photos Server - Development Wrap-Up Documentation

**Status**: Server Development Complete (Phase 1)  
**Date**: October 2024  
**Version**: 1.0.0

## Overview

This documentation provides a comprehensive analysis of the Lumilio Photos server implementation, covering major processes and architectural decisions. The server is built with Go, using PostgreSQL for storage, River for job queue management, and integrates with ML services for intelligent photo processing.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Processes](#core-processes)
   - [Inbox Upload Process](./01-inbox-upload.md)
   - [Directory Scanning & File Synchronization](./02-directory-scanning.md)
   - [Asset Processing Pipeline](./03-asset-processing.md)
   - [Database Operations](./04-database-operations.md)
3. [System Components](#system-components)
4. [Development Status](#development-status)

## Architecture Overview

### Technology Stack

- **Backend Framework**: Gin (Go web framework)
- **Database**: PostgreSQL with pgvector extension
- **Queue System**: River (PostgreSQL-based job queue)
- **Storage**: Multi-strategy repository system (Date-based, Flat, CAS)
- **ML Integration**: gRPC-based CLIP and classification services
- **API Documentation**: Swagger/OpenAPI

### High-Level Architecture

```
┌─────────────┐
│   Client    │
│  (Web/App)  │
└──────┬──────┘
       │ HTTP REST API
       ▼
┌─────────────────────────────────────┐
│         Gin Router + Handlers       │
│  (Asset, Album, Auth, Repository)   │
└──────────────┬──────────────────────┘
               │
     ┌─────────┴─────────┐
     ▼                   ▼
┌──────────┐      ┌────────────┐
│ Services │      │   Queue    │
│  Layer   │      │  (River)   │
└────┬─────┘      └─────┬──────┘
     │                  │
     │         ┌────────┴────────┐
     │         ▼                 ▼
     │  ┌───────────┐    ┌──────────────┐
     │  │  Workers  │    │  Processors  │
     │  └─────┬─────┘    └──────┬───────┘
     │        │                 │
     ▼        ▼                 ▼
┌──────────────────────────────────────┐
│         PostgreSQL Database          │
│    (Assets, Users, Repositories,     │
│     Embeddings, File Records)        │
└──────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│      Storage Repository System       │
│   (Directory Manager, Staging Mgr)   │
└──────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│        ML Service (gRPC)             │
│   (CLIP Embeddings, Classification)  │
└──────────────────────────────────────┘
```

## Core Processes

### 1. Inbox Upload Process

The upload process handles asset ingestion through a multi-stage pipeline:

1. **HTTP Request**: Client uploads file via multipart/form-data
2. **Staging**: File is temporarily stored in `.lumilio/staging/incoming/`
3. **Queue**: Upload job is enqueued to River queue
4. **Processing**: Worker processes the asset (deduplication, metadata extraction)
5. **Storage**: File is moved to repository inbox based on storage strategy
6. **Database**: Asset metadata and relationships are recorded

**Details**: See [01-inbox-upload.md](./01-inbox-upload.md)

### 2. Directory Scanning & File Synchronization

Two-tier system for keeping the database in sync with the filesystem:

1. **Real-time Watcher**: fsnotify-based monitoring for immediate change detection
2. **Daily Reconciliation**: Full filesystem walk to catch any missed changes

**Details**: See [02-directory-scanning.md](./02-directory-scanning.md)

### 3. Asset Processing Pipeline

Type-specific processing for photos, videos, and audio:

- **Photo Processing**: EXIF extraction, thumbnail generation, CLIP embeddings, smart classification
- **Video Processing**: Transcoding, thumbnail extraction, metadata parsing
- **Audio Processing**: Metadata extraction, waveform generation

**Details**: See [03-asset-processing.md](./03-asset-processing.md)

### 4. Database Operations

Service layer provides abstraction over database queries:

- **SQLC-generated queries**: Type-safe database access
- **Service layer**: Business logic and transaction management
- **Migration system**: Atlas for schema + River for queue tables

**Details**: See [04-database-operations.md](./04-database-operations.md)

## System Components

### Storage System

**Location**: `server/internal/storage/`

The storage system manages physical file organization:

- **DirectoryManager**: Creates and validates repository structure
- **StagingManager**: Handles temporary file staging and commits
- **RepositoryManager**: High-level repository lifecycle management
- **Repository Config**: YAML-based configuration per repository

**Key Features**:
- Multiple storage strategies (Date, Flat, CAS)
- Protected system directories (`.lumilio/`)
- User-managed content areas
- Trash system with recovery
- Duplicate filename handling

### Queue System

**Location**: `server/internal/queue/`

River-based job queue provides reliable background processing:

- **Workers**: ProcessAssetWorker, ProcessClipWorker
- **Dispatchers**: CLIP batch dispatcher for efficient ML inference
- **Job Types**: Asset processing, CLIP embedding, classification

**Key Features**:
- PostgreSQL-backed (ACID guarantees)
- Configurable concurrency per queue
- Batch processing for ML operations
- Automatic retry with exponential backoff

### Service Layer

**Location**: `server/internal/service/`

Services provide business logic abstraction:

- **AssetService**: Asset lifecycle, thumbnails, metadata
- **AuthService**: User authentication and authorization
- **AlbumService**: Album management and asset relationships
- **MLService**: ML model integration (CLIP, classification)

### Processors

**Location**: `server/internal/processors/`

Processors handle type-specific asset processing:

- **AssetProcessor**: Orchestrates the processing pipeline
- **PhotoProcessor**: Photo-specific operations
- **VideoProcessor**: Video transcoding and extraction
- **AudioProcessor**: Audio metadata and waveforms

### API Layer

**Location**: `server/internal/api/`

RESTful API with Swagger documentation:

- **Handlers**: Asset, Album, Auth, Repository endpoints
- **Middleware**: Authentication, logging, CORS
- **Response Format**: Standardized JSON responses

## Development Status

### ✅ Completed Features

1. **Core Infrastructure**
   - PostgreSQL database with migrations
   - River queue system
   - Repository management
   - Storage system with multiple strategies

2. **Upload Pipeline**
   - Single and batch upload endpoints
   - Staging and commit workflow
   - Deduplication by content hash
   - Queue-based processing

3. **Asset Processing**
   - Photo processing with EXIF extraction
   - Thumbnail generation (multiple sizes)
   - CLIP embeddings via gRPC
   - Smart classification (species detection)
   - Video and audio support

4. **File Synchronization**
   - Real-time file watching with fsnotify
   - Daily reconciliation scanner
   - Orphaned record cleanup
   - Hash-based change detection

5. **API & Documentation**
   - RESTful API design
   - Swagger/OpenAPI documentation
   - Comprehensive error handling
   - Authentication middleware

6. **Testing**
   - Unit tests for core components
   - Integration tests for workflows
   - Test coverage for storage system

### 🔄 Current Limitations

1. **Scalability**
   - Single-instance design (no distributed processing yet)
   - CLIP batch dispatcher limited to one worker for efficiency
   - File watching doesn't scale to millions of files

2. **ML Integration**
   - Requires external ML service (not bundled)
   - No fallback when ML service unavailable
   - Classification limited to pre-trained models

3. **User Management**
   - Basic authentication only
   - No OAuth/SSO integration
   - Single-tenant design

### 🚀 Future Enhancements

1. **Performance**
   - Distributed queue workers
   - CDN integration for thumbnails
   - Database read replicas
   - Redis caching layer

2. **Features**
   - Face recognition and clustering
   - Custom ML model training
   - Advanced search (natural language)
   - Mobile app support

3. **Deployment**
   - Docker orchestration (Kubernetes)
   - Horizontal scaling support
   - Cloud storage backends (S3, GCS)
   - Multi-region deployment

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 15+ with pgvector extension
- River CLI for migrations
- Optional: ML service for CLIP features

### Running the Server

```bash
# Navigate to server directory
cd server

# Set up environment
cp .env.development.example .env.development
# Edit .env.development with your configuration

# Install dependencies
go mod download

# Run migrations
river migrate-up --database-url "$DATABASE_URL"

# Start server
go run cmd/main.go
```

### Configuration

Key environment variables:

- `DATABASE_URL`: PostgreSQL connection string
- `STORAGE_PATH`: Base path for repositories
- `ML_SERVICE_ADDR`: gRPC address for ML service
- `CLIP_ENABLED`: Enable/disable CLIP processing

## Related Documentation

- [Existing Developer Docs](../docs/en/developer-documentation/)
- [Storage Strategies Guide](../server/docs/storage-strategies.md)
- [Queue System README](../server/internal/queue/README.md)
- [Sync System README](../server/internal/sync/README.md)
- [Storage System README](../server/internal/storage/README.md)

## Contact & Support

- **Repository**: [EdwinZhanCN/Lumilio-Photos](https://github.com/EdwinZhanCN/Lumilio-Photos)
- **Issues**: GitHub Issues
- **License**: GPLv3.0

---

*This documentation was created as part of the server development wrap-up to provide comprehensive insights into the system architecture and implementation details.*
