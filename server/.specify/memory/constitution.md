<!--
  Sync Impact Report
  ==================
  Version change: None → 1.0.0 (initial ratification)
  Modified principles: N/A (initial version)
  Added sections: All sections (initial version)
  Removed sections: N/A
  Templates requiring updates:
    ✅ plan-template.md - Reviewed, no constitution-specific gates found
    ✅ spec-template.md - Reviewed, compatible with prioritization principles
    ✅ tasks-template.md - Reviewed, compatible with independent delivery principles
    ⚠ agent-file-template.md - Will be updated when first feature is planned
  Follow-up TODOs: None
-->

# Lumilio Photos Constitution

## Core Principles

### I. Independent User Value Delivery

Every feature MUST be structured as prioritized, independently deliverable user stories. Each user story (P1, P2, P3, etc.) MUST:

- Provide standalone value to users without requiring other stories
- Be independently testable and deployable
- Have clear acceptance criteria that can be validated in isolation
- Enable incremental delivery (P1 → deploy → P2 → deploy → P3)

**Rationale**: This ensures MVP can be delivered quickly and each increment adds measurable user value. Prevents "big bang" releases where value is only realized after completing multiple interdependent features.

### II. Queue-Based Async Processing (NON-NEGOTIABLE)

All long-running asset processing operations MUST use asynchronous job queue:

- Video transcoding, thumbnail generation, EXIF extraction MUST be queued
- AI processing (OCR, face detection, embeddings, descriptions) MUST be queued
- Use River job queue framework with proper error handling and retries
- Workers MUST be idempotent and handle failures gracefully
- Job state MUST be visible and queryable via API

**Rationale**: Asset processing is resource-intensive and unreliable. Synchronous processing blocks requests, causes timeouts, and provides poor UX. Queue-based processing ensures scalability, reliability, and better user experience.

### III. AI-First Search & Discovery

All user-facing features MUST prioritize AI-powered search and discovery:

- Text search MUST use semantic embeddings (pgvector) not just keyword matching
- Assets MUST be enriched with AI descriptions, OCR text, and face metadata
- Search relevance MUST be continuously improved based on user feedback
- AI processing pipeline MUST be extensible for new AI capabilities

**Rationale**: The core value proposition is AI-powered photo discovery. Without semantic search and rich metadata, the product is just another file storage system.

### IV. Storage Format Standardization

All assets MUST be stored in standardized, web-compatible formats:

- Original files MUST be preserved and never modified
- Web-optimized derivatives MUST be generated (MP4/H.264 for video, JPEG for images)
- Metadata MUST be extracted and stored in structured database fields
- File organization MUST be deterministic and reversible (hash-based paths)

**Rationale**: Ensures long-term preservation of originals while providing optimal web performance. Prevents vendor lock-in and enables migration.

### V. API-First Design

All features MUST expose functionality via RESTful API before UI implementation:

- API contracts MUST be defined using Swagger/OpenAPI specifications
- Endpoints MUST be documented with examples and error cases
- API design MUST support frontend as primary client (not just admin console)
- Versioning MUST be considered for breaking changes

**Rationale**: Clean API enables multiple clients (web, mobile, external integrations). Forces thinking about data flow and contracts before UI implementation.

## Development Standards

### Code Organization

- Go packages MUST follow standard project layout (cmd/, internal/, pkg/)
- Business logic MUST reside in internal/service/ layer (not handlers)
- Database access MUST use sqlc-generated types and queries (no raw SQL in handlers)
- Job workers MUST be in internal/queue/ with clear input/output contracts

### Testing Requirements

- Services MUST include table-driven unit tests for core business logic
- Asset processors MUST include integration tests with sample media files
- API contracts MUST be validated against OpenAPI spec
- Queue workers MUST be tested for idempotency and error handling

**Rationale**: Asset processing has many edge cases (corrupt files, huge files, exotic formats). Tests are essential given the complexity.

### Error Handling

- All errors MUST be logged with context (asset ID, operation, params)
- Client-facing errors MUST be user-friendly (not internal stack traces)
- Worker errors MUST include retry metadata and actionable debugging info
- API errors MUST follow RFC 7807 Problem Details for HTTP APIs format

**Rationale**: Debugging async job failures is difficult without proper error context. Users need clear guidance when operations fail.

### Observability (NON-NEGOTIABLE)

- All operations MUST emit structured logs (zap logger)
- Job queue metrics MUST be monitored (job rate, error rate, queue depth)
- Processing performance MUST be tracked (p50, p95, p99 latencies)
- Asset processing pipeline MUST include progress reporting

**Rationale**: Async processing is invisible to users. Observability is required to understand system health and debug issues.

## Security & Privacy

### Data Protection

- User assets MUST be isolated at storage and database level
- API access MUST be authenticated via JWT tokens
- Sensitive metadata (GPS, device info) MUST be handled per user preferences
- Original files MUST never be exposed directly (always through API)

### External AI Services

- AI service API keys MUST be configurable via environment variables
- Data sent to external AI services MUST be disclosed to users
- AI features MUST be gracefully degradable if services are unavailable
- Fallback behavior MUST be explicit (skip feature vs. block processing)

**Rationale**: External AI dependencies introduce availability and privacy risks. System must function without them.

## Governance

### Amendment Process

1. Proposals MUST be documented with rationale and impact analysis
2. Changes MUST update version number according to semantic versioning
3. All dependent artifacts (templates, docs) MUST be updated synchronously
4. Breaking changes require migration plan for existing code

### Versioning

- **MAJOR**: Principle removal or backward-incompatible governance changes
- **MINOR**: New principle added or existing principle materially expanded
- **PATCH**: Clarifications, wording improvements, non-semantic refinements

### Compliance Review

- All feature specifications MUST reference relevant constitution principles
- Implementation plans MUST document any principle violations with justification
- Code reviews MUST verify compliance with core principles
- Violations MUST be approved by lead maintainer and documented in plan.md

**Version**: 1.0.0 | **Ratified**: 2025-01-13 | **Last Amended**: 2025-01-13
