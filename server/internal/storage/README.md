## Storage Repository System

### Phase 1.2: Directory Structure Setup & System Management

This implementation provides the foundational directory management system for Lumilio repositories, establishing proper file organization and protection mechanisms as outlined in Phase 1.2 of the Storage System Refactor project.

### Repository Directory Structure

```toml
repository-root/
├── .lumilio/                # System-managed (protected)
│   ├── assets/
│   │   ├── thumbnails/     # Generated thumbnails by size
│   │   │   ├── 150/        # 150px thumbnails
│   │   │   ├── 300/        # 300px thumbnails
│   │   │   └── 1024/       # Large previews
│   │   ├── videos/         # Transcoded video files
│   │   │   └── web/        # MP4/H.264 ≤1080p (target) transcodes
│   │   ├── audios/         # Transcoded audio files
│   │   │   └── web/        # transcoded MP3
│   ├── staging/            # Upload staging area
│   │   ├── incoming/       # Files being uploaded
│   │   └── failed/         # Failed upload attempts
│   ├── temp/              # Temporary processing files
│   ├── trash/             # Soft-delete assets
│   ├── logs/              # Application and operation logs
│   └── backups/           # Config version backups
├── .lumiliorepo           # Repository configuration
├── inbox/                 # Structured uploads (protected)
│   ├── 2024/              # Date-based or hash-based
│   │   ├── 01/            # Depends on storage strategy
│   │   └── 02/
│   └── ...
└── [user-space]/          # User-managed files (unprotected)
    ├── Family Photos/     # User can organize however
    ├── Vacations/         # they want
    └── ...
```

### Architecture Components

#### 1. DirectoryManager
- **File**: `directory_manager.go`
- **Purpose**: Handles physical directory structure and system management
- **Key Features**:
  - Creates and validates repository directory structure
  - Protects system directories with appropriate permissions
  - Manages staging, temporary, and trash operations
  - Repairs missing directories and structure issues

#### 2. StagingManager
- **File**: `staging_manager.go` 
- **Purpose**: Handles staging operations with repository configuration integration
- **Key Features**:
  - Creates staging files for uploads
  - Commits files to inbox based on storage strategy (date/flat/CAS)
  - Handles duplicate filename resolution
  - Integrates with repository configuration for path resolution

#### 3. RepositoryManager (Updated)
- **File**: `repo_manager.go`
- **Purpose**: High-level repository management using directory and staging managers
- **Changes**:
  - Delegates directory operations to DirectoryManager
  - Provides access to StagingManager for file operations
  - Removed duplicate file management functions

### Protection Mechanisms

#### System Directory Protection
- `.lumilio/` directories: Protected from user modification
- `inbox/` directory: Read-only for users, managed by application
- Staging areas: Application-only access (700 permissions)
- User areas: Full user access (777 permissions)

#### Path Validation
- Protected paths are automatically detected
- Prevents user modification of system-managed content
- Allows full freedom in user-managed areas

### File Management Workflows

#### Staging Workflow
1. Create staging file in `.lumilio/staging/incoming/`
2. Upload/process file content
3. Validate file integrity
4. Commit to final destination (inbox or user area)
5. Cleanup staging files based on age

#### Trash System
- Soft delete with metadata preservation
- 30-day default recovery window
- Automatic purging of old items
- Recovery to original location with metadata

#### Temporary File Management
- Purpose-based temporary file creation
- Automatic cleanup based on age
- Isolated processing workspace

### Storage Strategies

#### Date Strategy (default)
- Files stored in `inbox/YYYY/MM/` structure
- Month-based organization for easy browsing

#### Flat Strategy
- All files directly in `inbox/` directory
- Simple, flat organization

#### Content-Addressed Storage (CAS)
- Files stored in `inbox/aa/bb/cc/hash.ext` structure
- Deduplication through content hashing
- Falls back to date strategy if hash unavailable

### Usage Examples

```go
// Create directory manager
dirManager := NewDirectoryManager()

// Create repository structure
err := dirManager.CreateStructure("/path/to/repo")

// Validate existing structure
validation, err := dirManager.ValidateStructure("/path/to/repo")

// Repair missing directories
err := dirManager.RepairStructure("/path/to/repo")

// Create staging manager
stagingManager := NewStagingManager()

// Stage and commit file to inbox
stagingFile, err := stagingManager.CreateStagingFile(repoPath, "photo.jpg")
err = stagingManager.CommitStagingFileToInbox(stagingFile, contentHash)

// Resolve inbox path based on strategy
finalPath, err := stagingManager.ResolveInboxPath(repoPath, "photo.jpg", hash)
```

**Example Repository Config File**

```yaml
# Lumilio Repository Configuration
# This file defines the configuration for a Lumilio photo repository

version: "1.0"
id: "550e8400-e29b-41d4-a716-446655440000"
name: "Family Photos"
created_at: "2024-01-15T10:30:00Z"

# Storage strategy determines how files are organized in the inbox
# - "date": Organize by date (YYYY/MM structure)
# - "cas": Content-addressed storage (hash-based organization)
# - "flat": All files in a single directory
storage_strategy: "date"

# Synchronization settings for file system monitoring
sync_settings:
  # How often to perform quick metadata scans (file size, modification time)
  quick_scan_interval: "5m"

  # How often to perform full content verification scans
  full_scan_interval: "30m"

  # File patterns to ignore during scanning
  ignore_patterns:
    - ".DS_Store"
    - "Thumbs.db"
    - "ehthumbs.db"
    - "Icon?"
    - "Icon\r"
    - "*.tmp"
    - "*.temp"
    - "*.part"
    - "*.partial"
    - "*.wbk"
    - "*.bak"
    - "*.orig"
    - "*~"
    - "~$*"
    - "*.swp"
    - "*.swo"
    - ".*.swp"
    - ".lumilio"
    - ".Trash"
    - ".Trashes"
    - ".fseventsd"
    - ".Spotlight-V100"
    - ".TemporaryItems"
    - "lost+found"
    - "desktop.ini"
    - "*._*"
    - "npm-debug.log"
    - "yarn-error.log"

# Repository-specific local settings
local_settings:
  # Whether to preserve original filenames when storing files
  preserve_original_filename: true

  # How to handle duplicate filenames:
  # - "rename": Add (1), (2) suffix
  # - "uuid": Append UUID to filename
  # - "overwrite": Replace existing file
  handle_duplicate_filenames: "uuid"

  # Maximum file size in bytes (0 = no limit), KB unit
  max_file_size: 0

  # Whether to compress files before storage
  compress_files: false

  # Whether to create backup copies of files
  create_backups: true

  # Path where backup copies should be stored (only used if create_backups is true)
  # Can be external drive, NAS, cloud mount, etc.
  backup_path: "/mnt/external-backup"
```
