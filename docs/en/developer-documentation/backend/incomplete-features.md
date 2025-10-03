# Incomplete Features and Functions

*Last Updated: 2024*  
*Author: Edwin Zhan, documented by AI*

## Overview

This document tracks incomplete features, partial implementations, and known limitations in the Lumilio Photos backend server. Items are categorized by priority and component.

## Critical Priority

### 1. Authentication System
**Status**: Placeholder only  
**Component**: `server/internal/service/auth_service.go`  
**Current State**:
- Basic user ID extraction exists
- No actual authentication logic
- All endpoints publicly accessible

**Required Work**:
- [ ] Implement user registration
- [ ] Add password hashing (bcrypt/argon2)
- [ ] Implement JWT token generation
- [ ] Add JWT token validation middleware
- [ ] Create login/logout endpoints
- [ ] Add refresh token mechanism
- [ ] Implement password reset flow

**Impact**: Cannot deploy to production without authentication

---

### 2. Authorization and Access Control
**Status**: Not implemented  
**Component**: Middleware layer (needs creation)  
**Current State**: No access control checks

**Required Work**:
- [ ] Design RBAC model (roles: admin, user, guest)
- [ ] Implement authorization middleware
- [ ] Add ownership checks for assets
- [ ] Implement album sharing permissions
- [ ] Add API key authentication for clients
- [ ] Create permission management API

**Impact**: Security risk, no multi-user support

---

### 3. Transaction Management
**Status**: Incomplete  
**Component**: All services  
**Current State**:
- Database operations not wrapped in transactions
- No rollback on partial failures
- Orphaned records possible

**Required Work**:
- [ ] Wrap asset processing in database transactions
- [ ] Add storage rollback for failed DB operations
- [ ] Implement cleanup for orphaned staging files
- [ ] Add retry logic with exponential backoff
- [ ] Create transaction helpers for common patterns

**Impact**: Data consistency issues, orphaned files

---

### 4. Error Handling and Recovery
**Status**: Basic  
**Component**: All layers  
**Current State**:
- Errors logged but not always handled
- No user-friendly error messages
- No error recovery mechanisms

**Required Work**:
- [ ] Define error types and codes
- [ ] Implement structured error responses
- [ ] Add context to errors (operation, file, user)
- [ ] Create error recovery procedures
- [ ] Add error reporting to monitoring system
- [ ] Implement circuit breakers for external services

**Impact**: Poor user experience, difficult debugging

---

## High Priority

### 5. Advanced Search and Filtering
**Status**: Basic implementation  
**Component**: `server/internal/service/asset_service.go`, API handlers  
**Current State**:
- Simple pagination exists
- Basic asset listing
- No filtering or search

**Required Work**:
- [ ] Add filters (date range, type, rating, etc.)
- [ ] Implement full-text search on metadata
- [ ] Add sorting options (date, name, size, rating)
- [ ] Create search by GPS location/region
- [ ] Implement tag-based filtering
- [ ] Add combined filter queries

**Impact**: Poor user experience with large libraries

---

### 6. Semantic Search with Vector Database
**Status**: Database ready, no API  
**Component**: ML Service, API layer  
**Current State**:
- Embeddings stored in pgvector
- No search endpoint
- No UI integration

**Required Work**:
- [ ] Create vector similarity search API endpoint
- [ ] Add text-to-vector search (text prompt → similar images)
- [ ] Implement image-to-image search
- [ ] Add configurable similarity threshold
- [ ] Create search result ranking algorithm
- [ ] Optimize vector index for performance

**Impact**: Missing key differentiating feature

---

### 7. Video Transcoding and Optimization
**Status**: Basic implementation  
**Component**: `server/internal/processors/video_processor.go`  
**Current State**:
- Basic transcoding exists
- No format optimization
- No adaptive streaming

**Required Work**:
- [ ] Add adaptive bitrate streaming (HLS/DASH)
- [ ] Implement multiple quality levels (480p, 720p, 1080p)
- [ ] Add smart transcoding (skip if already optimal)
- [ ] Create transcoding progress tracking
- [ ] Add thumbnail timeline generation
- [ ] Implement video metadata extraction improvements

**Impact**: Poor video playback experience, storage waste

---

### 8. Deduplication Improvements
**Status**: Hash-based only  
**Component**: `server/internal/service/asset_service.go`  
**Current State**:
- Only exact hash matching
- No perceptual hashing
- No near-duplicate detection

**Required Work**:
- [ ] Implement perceptual hashing (pHash, dHash)
- [ ] Add near-duplicate detection with similarity threshold
- [ ] Create duplicate management UI
- [ ] Add duplicate merging (keep best quality)
- [ ] Implement smart duplicate selection
- [ ] Add manual duplicate marking

**Impact**: Storage waste, duplicate management burden

---

### 9. Backup and Recovery
**Status**: Config mentions, not implemented  
**Component**: Storage system  
**Current State**:
- Repository config has `backup_path` field
- No backup logic implemented
- No restore procedures

**Required Work**:
- [ ] Implement automated backup scheduling
- [ ] Add incremental backup support
- [ ] Create backup verification
- [ ] Implement restore procedures
- [ ] Add backup to cloud storage (S3, etc.)
- [ ] Create backup monitoring and alerts

**Impact**: Data loss risk

---

## Medium Priority

### 10. Smart Albums and Collections
**Status**: Not implemented  
**Component**: Album service  
**Current State**: Only manual albums exist

**Required Work**:
- [ ] Design rule-based album system
- [ ] Implement auto-updating albums (rules like "all photos from 2024")
- [ ] Add nested album/collection support
- [ ] Create album templates
- [ ] Implement album sharing
- [ ] Add collaborative albums

**Impact**: Limited organization capabilities

---

### 11. Batch Operations API
**Status**: Partially implemented  
**Component**: All services  
**Current State**:
- Batch upload exists
- No other batch operations

**Required Work**:
- [ ] Add batch delete
- [ ] Implement batch move to album
- [ ] Add batch metadata update
- [ ] Create batch rating/tagging
- [ ] Implement batch download (as ZIP)
- [ ] Add batch operation status tracking

**Impact**: Inefficient bulk operations

---

### 12. Asset Versioning
**Status**: Not implemented  
**Component**: Asset service, database  
**Current State**: No version tracking

**Required Work**:
- [ ] Design version schema
- [ ] Implement version creation on edits
- [ ] Add version comparison
- [ ] Create version restoration
- [ ] Implement version pruning (keep N versions)
- [ ] Add version metadata tracking

**Impact**: Cannot undo edits, no edit history

---

### 13. Scheduled Jobs and Maintenance
**Status**: Manual only  
**Component**: Queue system  
**Current State**:
- No scheduled job support
- Manual cleanup required

**Required Work**:
- [ ] Add cron-like scheduled jobs to River
- [ ] Implement staging cleanup job (daily)
- [ ] Add trash purge job (30 days)
- [ ] Create thumbnail regeneration job
- [ ] Implement database vacuum scheduling
- [ ] Add statistics generation job

**Impact**: Manual maintenance burden

---

### 14. Asset Editing Pipeline
**Status**: Not started  
**Component**: New processors needed  
**Current State**: No editing support

**Required Work**:
- [ ] Design non-destructive editing system
- [ ] Implement crop, rotate, flip operations
- [ ] Add filters and adjustments (brightness, contrast, etc.)
- [ ] Create editing history tracking
- [ ] Implement undo/redo
- [ ] Add preset saving and application

**Impact**: Must use external tools for editing

---

### 15. Sharing and Collaboration
**Status**: Not started  
**Component**: New service needed  
**Current State**: No sharing capabilities

**Required Work**:
- [ ] Design sharing model (links, users, permissions)
- [ ] Implement share link generation
- [ ] Add password-protected shares
- [ ] Create expiring shares
- [ ] Implement shared album collaboration
- [ ] Add commenting on shared assets

**Impact**: Limited collaboration capabilities

---

### 16. Mobile API Optimization
**Status**: Not optimized  
**Component**: API layer  
**Current State**: Desktop-focused API

**Required Work**:
- [ ] Add mobile-specific thumbnail sizes
- [ ] Implement progressive image loading
- [ ] Add bandwidth-aware responses
- [ ] Create offline support indicators
- [ ] Implement delta sync API
- [ ] Add push notification infrastructure

**Impact**: Poor mobile experience

---

### 17. Performance Monitoring
**Status**: Not implemented  
**Component**: Observability layer  
**Current State**: Basic logging only

**Required Work**:
- [ ] Add Prometheus metrics collection
- [ ] Implement request duration tracking
- [ ] Add queue depth monitoring
- [ ] Create storage usage metrics
- [ ] Implement database query performance tracking
- [ ] Add custom business metrics (uploads/day, etc.)

**Impact**: No performance visibility

---

## Low Priority

### 18. Face Detection and Recognition
**Status**: Not started  
**Component**: ML service integration  
**Current State**: No face features

**Required Work**:
- [ ] Integrate face detection model
- [ ] Implement face clustering
- [ ] Add face labeling UI
- [ ] Create person albums
- [ ] Implement face search
- [ ] Add privacy controls for face data

**Impact**: Missing popular feature

---

### 19. OCR and Text Recognition
**Status**: Not started  
**Component**: ML service integration  
**Current State**: No text extraction

**Required Work**:
- [ ] Integrate OCR engine (Tesseract/cloud)
- [ ] Implement text extraction from images
- [ ] Add text search in images
- [ ] Create document management features
- [ ] Implement receipt/document categorization

**Impact**: Limited document search

---

### 20. Map View and Geospatial Features
**Status**: GPS extracted but not used  
**Component**: Frontend + API  
**Current State**: GPS coordinates stored, no map

**Required Work**:
- [ ] Create map view API
- [ ] Implement location-based clustering
- [ ] Add map browsing UI
- [ ] Create location-based albums
- [ ] Implement GPS privacy controls
- [ ] Add location tagging for photos without GPS

**Impact**: GPS data not utilized

---

### 21. Timeline and Memories
**Status**: Not started  
**Component**: New feature  
**Current State**: No timeline features

**Required Work**:
- [ ] Design timeline algorithm
- [ ] Implement "On This Day" feature
- [ ] Create automatic memory collections
- [ ] Add timeline browsing UI
- [ ] Implement importance scoring
- [ ] Create shareable memory cards

**Impact**: Missing engagement feature

---

### 22. Third-Party Integrations
**Status**: Not started  
**Component**: Integration layer  
**Current State**: Standalone system

**Required Work**:
- [ ] Add Google Photos import
- [ ] Implement iCloud sync
- [ ] Create IFTTT/Zapier webhooks
- [ ] Add social media sharing
- [ ] Implement cloud backup integration (Backblaze, etc.)
- [ ] Create calendar integration (photo events)

**Impact**: Migration and workflow friction

---

### 23. Advanced ML Features
**Status**: CLIP only  
**Component**: ML service  
**Current State**: Only CLIP embeddings and classification

**Required Work**:
- [ ] Add image quality assessment
- [ ] Implement automatic photo enhancement
- [ ] Add duplicate detection ML model
- [ ] Create automatic tagging improvements
- [ ] Implement scene detection
- [ ] Add aesthetic scoring

**Impact**: Limited AI capabilities

---

### 24. Multi-Repository Management
**Status**: Single repo focus  
**Component**: Repository manager  
**Current State**: One repository per user expected

**Required Work**:
- [ ] Add repository switching UI
- [ ] Implement cross-repository search
- [ ] Create repository synchronization
- [ ] Add repository comparison tools
- [ ] Implement repository merge utilities

**Impact**: Power users need multiple repos

---

### 25. Customization and Themes
**Status**: Not started  
**Component**: Configuration system  
**Current State**: No customization

**Required Work**:
- [ ] Add user preference storage
- [ ] Implement theme system
- [ ] Create customizable layouts
- [ ] Add workflow customization
- [ ] Implement plugin architecture
- [ ] Create user scripts support

**Impact**: Limited personalization

---

## Testing Gaps

### Unit Testing
- [ ] Processor layer comprehensive tests
- [ ] Service layer edge case tests
- [ ] API handler tests with mocks
- [ ] Error path testing
- [ ] Concurrent operation tests

### Integration Testing
- [ ] End-to-end upload flow
- [ ] Queue processing reliability
- [ ] ML service integration
- [ ] Storage failure scenarios
- [ ] Database transaction rollback

### Performance Testing
- [ ] Load testing (1000+ concurrent uploads)
- [ ] Stress testing (resource exhaustion)
- [ ] Soak testing (24+ hour runs)
- [ ] Storage I/O benchmarking
- [ ] Database query performance

### Security Testing
- [ ] Authentication bypass attempts
- [ ] SQL injection testing
- [ ] File upload security
- [ ] Path traversal vulnerabilities
- [ ] DoS attack resistance

---

## Infrastructure and DevOps

### Deployment
- [ ] Kubernetes manifests
- [ ] Helm charts
- [ ] Terraform/IaC templates
- [ ] Multi-region deployment guide
- [ ] Blue-green deployment support

### Monitoring and Alerting
- [ ] Health check endpoints
- [ ] Readiness/liveness probes
- [ ] Alert rules definition
- [ ] Incident response playbooks
- [ ] Performance dashboards

### CI/CD
- [ ] Automated testing pipeline
- [ ] Code quality gates
- [ ] Automated dependency updates
- [ ] Release automation
- [ ] Rollback procedures

---

## Documentation Gaps

### Developer Documentation
- [ ] Architecture decision records (ADRs)
- [ ] Database schema documentation
- [ ] API versioning strategy
- [ ] Contributing guidelines
- [ ] Code style guide

### Operational Documentation
- [ ] Deployment guide
- [ ] Configuration reference
- [ ] Troubleshooting guide
- [ ] Disaster recovery procedures
- [ ] Capacity planning guide

### User Documentation
- [ ] Installation guide
- [ ] Configuration tutorial
- [ ] Feature documentation
- [ ] FAQ
- [ ] Video tutorials

---

## Prioritization Framework

### Priority Definitions

**Critical**: Must have for production deployment
- Security issues
- Data integrity risks
- System stability

**High**: Significantly impacts user experience or operations
- Core feature gaps
- Performance bottlenecks
- Operational complexity

**Medium**: Nice to have, improves experience
- Enhancement features
- Convenience improvements
- Advanced capabilities

**Low**: Future improvements, niche use cases
- Specialized features
- Experimental capabilities
- Power user tools

### Estimated Effort

- 🟢 Small: 1-5 days
- 🟡 Medium: 1-2 weeks  
- 🔴 Large: 2-4 weeks
- ⚫ Extra Large: 1+ months

### Dependencies

Many items have dependencies:
- Face detection requires ML service expansion
- Semantic search needs vector search API
- Batch operations need transaction management
- Sharing requires authentication

See [Future Roadmap](./future-roadmap.md) for sequenced development plan.
