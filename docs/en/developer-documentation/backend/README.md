# Backend Documentation Index

*Last Updated: 2024*  
*Author: Edwin Zhan, documented by AI*

## Overview

This directory contains comprehensive documentation for the Lumilio Photos backend server, including development status, architectural deep-dives, and future planning.

## Documentation Structure

### 📊 Project Status

#### [Development Status](./development-status.md)
Complete overview of the current server implementation state:
- ✅ Components that are production-ready
- ⚠️ Features that need work
- ❌ Missing functionality
- 📈 Performance characteristics
- 🔒 Security status
- 📝 Testing coverage

**When to read**: Understanding what's complete and what needs work

---

#### [Incomplete Features](./incomplete-features.md)
Detailed tracking of partial implementations and gaps:
- Prioritized by Critical/High/Medium/Low
- Estimated effort for each item
- Dependencies between features
- Testing gaps
- Infrastructure needs

**When to read**: Planning development work, understanding limitations

---

#### [Future Roadmap](./future-roadmap.md)
Phased development plan for the next 12+ months:
- Phase 1: Production Readiness (2-3 months)
- Phase 2: Core Feature Enhancement (3-4 months)
- Phase 3: Advanced Features (4-6 months)
- Phase 4: Scalability and Enterprise (6+ months)
- Phase 5: Mobile and Offline Support (4+ months)
- Resource allocation recommendations
- Success metrics

**When to read**: Long-term planning, team coordination

---

### 🔧 Process Documentation

#### [Inbox Upload](./processes/inbox-upload.md)
Complete analysis of the asset upload workflow:
- HTTP request handling
- Background job processing
- Storage commit (3 strategies: date/flat/CAS)
- Type-specific processing (photos, videos, audio)
- ML integration with CLIP
- Error handling and recovery
- Performance benchmarks
- Troubleshooting guide

**When to read**: Debugging uploads, optimizing performance, understanding asset ingestion

---

#### [Directory Scanning](./processes/directory-scanning.md)
Two-tier file synchronization system:
- Real-time file watcher (fsnotify)
- Daily reconciliation scanner
- Protected vs user-managed areas
- Event processing and debouncing
- Batch processing and hash calculation
- Database schema for file tracking
- Configuration tuning
- Integration with asset management

**When to read**: Understanding file sync, debugging missing files, configuring repositories

---

#### [Database Operations](./processes/database-operations.md)
Comprehensive database architecture documentation:
- Schema design (assets, thumbnails, embeddings, etc.)
- Migration strategy (Atlas + River)
- SQLC code generation
- Connection pooling and management
- Query patterns and best practices
- Transaction handling
- Vector search with pgvector
- Performance optimization
- Backup and recovery

**When to read**: Schema changes, query optimization, troubleshooting database issues

---

#### [Asset Processing](./processors/asset-processor.md)
Deep dive into the asset processor architecture:
- Processor responsibilities and dependencies
- Workflow overview
- Sub-processors (photo, video, audio)
- ML service integration
- Error handling

**When to read**: Understanding asset processing pipeline, adding new asset types

---

## Quick Reference

### By Use Case

**I want to...**

- **Understand what's implemented**: Read [Development Status](./development-status.md)
- **Know what's missing**: Read [Incomplete Features](./incomplete-features.md)
- **Plan future work**: Read [Future Roadmap](./future-roadmap.md)
- **Debug upload issues**: Read [Inbox Upload](./processes/inbox-upload.md)
- **Understand file sync**: Read [Directory Scanning](./processes/directory-scanning.md)
- **Optimize database queries**: Read [Database Operations](./processes/database-operations.md)
- **Add asset processing**: Read [Asset Processing](./processors/asset-processor.md)
- **Deploy to production**: Read all Status docs + Process docs

---

### By Component

**API Layer**:
- [Inbox Upload - Phase 1](./processes/inbox-upload.md#phase-1-http-request-handling)
- [Development Status - API Layer](./development-status.md#api-layer-functional)

**Queue System**:
- [Inbox Upload - Phase 2](./processes/inbox-upload.md#phase-2-background-job-processing)
- [Development Status - Queue System](./development-status.md#queue-system)

**Storage System**:
- [Inbox Upload - Phase 3](./processes/inbox-upload.md#phase-3-storage-commit-inbox)
- [Directory Scanning](./processes/directory-scanning.md)
- [Development Status - Storage System](./development-status.md#storage-system)

**Database**:
- [Database Operations](./processes/database-operations.md)
- [Development Status - Database Layer](./development-status.md#database-layer)

**Asset Processing**:
- [Inbox Upload - Phase 4](./processes/inbox-upload.md#phase-4-type-specific-processing)
- [Asset Processing](./processors/asset-processor.md)

**ML Integration**:
- [Inbox Upload - Phase 5](./processes/inbox-upload.md#phase-5-ml-processing-optional)
- [Development Status - ML Integration](./development-status.md#ml-integration-functional)

---

### By Development Phase

**Phase 1: Production Readiness**
- Read: [Future Roadmap - Phase 1](./future-roadmap.md#phase-1-production-readiness-critical-path)
- Focus: Authentication, security, transactions, observability
- Priority: Critical

**Phase 2: Core Features**
- Read: [Future Roadmap - Phase 2](./future-roadmap.md#phase-2-core-feature-enhancement-3-4-months)
- Focus: Search, deduplication, video processing, albums
- Priority: High

**Phase 3: Advanced Features**
- Read: [Future Roadmap - Phase 3](./future-roadmap.md#phase-3-advanced-features-4-6-months)
- Focus: Face detection, maps, timeline, editing
- Priority: Medium

**Phase 4: Enterprise**
- Read: [Future Roadmap - Phase 4](./future-roadmap.md#phase-4-scalability-and-enterprise-6-months)
- Focus: Performance, HA, multi-tenancy, integrations
- Priority: Medium

---

## Architecture Diagrams

### High-Level System Architecture

```
┌─────────────┐
│   Client    │
│   (Web/App) │
└──────┬──────┘
       │ HTTP
       ↓
┌─────────────────────────────────────────────────┐
│              API Layer (Gin)                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │  Assets  │ │  Albums  │ │  Search  │ ...   │
│  │ Handler  │ │ Handler  │ │ Handler  │       │
│  └──────────┘ └──────────┘ └──────────┘       │
└────────┬─────────────────────┬──────────────────┘
         │                     │
         ↓                     ↓
┌──────────────────┐  ┌──────────────────┐
│  Queue (River)   │  │  Database (PG)   │
│                  │  │                  │
│  ┌────────────┐  │  │  ┌────────────┐ │
│  │ process_   │  │  │  │  assets    │ │
│  │ asset      │  │  │  │  albums    │ │
│  │            │  │  │  │  embeddings│ │
│  ├────────────┤  │  │  └────────────┘ │
│  │ process_   │  │  │                  │
│  │ clip       │  │  │  pgvector ext    │
│  └────────────┘  │  └──────────────────┘
└────────┬─────────┘
         │
         ↓
┌──────────────────────────────────────────┐
│         Workers & Processors             │
│  ┌────────────┐  ┌────────────┐         │
│  │   Asset    │  │    CLIP    │         │
│  │ Processor  │  │   Worker   │         │
│  │            │  │            │         │
│  │ ┌────────┐ │  │            │         │
│  │ │ Photo  │ │  │            │         │
│  │ │ Video  │ │  │            │         │
│  │ │ Audio  │ │  │            │         │
│  │ └────────┘ │  └────────────┘         │
│  └────────────┘                          │
└────┬───────────────────────┬─────────────┘
     │                       │
     ↓                       ↓
┌──────────────┐    ┌──────────────┐
│   Storage    │    │  ML Service  │
│              │    │   (gRPC)     │
│  .lumilio/   │    │              │
│  inbox/      │    │  CLIP Model  │
│  user-area/  │    └──────────────┘
└──────────────┘
```

### Upload Flow

```
Client → API → Staging → Queue → Worker → [Process] → Storage + DB → [ML]
  |       |       |         |       |         |          |     |      |
 Upload  Save   Fast     Enqueue Dequeue  Validate   Commit Create  CLIP
        File   Response    Job              Type     to Inbox Record
```

### Directory Scanning Flow

```
┌─────────────────────────────────────────────┐
│          User-Managed Files                 │
│     (repository/Photos/, etc.)              │
└──────────────┬──────────────────────────────┘
               │
        ┌──────┴──────┐
        │             │
        ↓             ↓
   ┌─────────┐   ┌──────────────┐
   │ Watcher │   │ Reconciliation│
   │ (Real-  │   │  (Daily Scan) │
   │  time)  │   └──────────────┘
   └────┬────┘          │
        │               │
        └───────┬───────┘
                ↓
         ┌─────────────┐
         │ file_records│
         │   (DB)      │
         └─────────────┘
```

---

## Common Tasks

### Adding a New Feature

1. Check [Incomplete Features](./incomplete-features.md) for existing plans
2. Review [Future Roadmap](./future-roadmap.md) for sequencing
3. Read relevant process docs for integration points
4. Update documentation when complete

### Debugging an Issue

1. Identify component (API, Queue, Storage, Database)
2. Read process doc for that component
3. Check [Development Status](./development-status.md) for known issues
4. Use troubleshooting guides in process docs

### Performance Optimization

1. Read [Database Operations - Performance](./processes/database-operations.md#performance-optimization)
2. Check [Inbox Upload - Benchmarks](./processes/inbox-upload.md#performance-characteristics)
3. Review [Directory Scanning - Tuning](./processes/directory-scanning.md#configuration-tuning)
4. Monitor and measure changes

### Production Deployment

**Pre-Deployment Checklist**:
- [ ] Review [Development Status - Production Concerns](./development-status.md#production-concerns)
- [ ] Complete [Future Roadmap - Phase 1](./future-roadmap.md#phase-1-production-readiness-critical-path)
- [ ] Set up monitoring (see [Development Status - Observability](./development-status.md#observability))
- [ ] Configure backups (see [Database Operations - Backup](./processes/database-operations.md#backup-and-recovery))
- [ ] Test failure scenarios

---

## Contributing

When updating documentation:

1. **Keep consistent**: Follow the structure and style of existing docs
2. **Be comprehensive**: Include examples, code snippets, and diagrams
3. **Stay current**: Update docs when code changes
4. **Cross-reference**: Link to related documentation
5. **Practical focus**: Include troubleshooting and real-world examples

---

## External Documentation

### Official Repository Docs
- [Storage System](../../../../server/internal/storage/README.md)
- [Sync System](../../../../server/internal/sync/README.md)
- [Queue System](../../../../server/internal/queue/README.md)
- [Database Migrations](../../../../server/internal/db/README.md)

### Business Diagrams
- [Upload Backend Flow](../business-diagram/upload-backend.md)
- [Upload Frontend Flow](../business-diagram/upload-frontend.md)

### Tech Stack
- [Tech Stack Overview](../techstack-overview.md)
- [Backend Technologies](../backend.md)
- [Frontend Technologies](../frontend.md)

---

## Getting Help

**For Development Questions**:
1. Search this documentation
2. Check code comments
3. Review existing issues on GitHub
4. Ask in team chat

**For Production Issues**:
1. Check troubleshooting guides
2. Review logs and metrics
3. Consult [Development Status - Known Issues](./development-status.md#known-issues-and-bugs)
4. Create incident report

---

## Maintenance

This documentation should be updated:
- **After major features**: Add process docs
- **After releases**: Update development status
- **Quarterly**: Review and update roadmap
- **When issues found**: Update known issues and troubleshooting

**Last Major Update**: 2024  
**Next Review**: TBD

---

## Feedback

Found an error or have a suggestion?
- Open an issue on GitHub
- Submit a PR with corrections
- Discuss in team meetings

Good documentation is a team effort. Thank you for contributing!
