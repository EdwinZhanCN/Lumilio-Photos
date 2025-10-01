## Storage Repository System

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
│   └── trash/             # Soft-delete assets
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
