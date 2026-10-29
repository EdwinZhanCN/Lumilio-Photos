// Package service holds the business logic behind the HTTP API.
//
// Each domain exposes a narrow interface with a constructor: authentication,
// sessions, MFA, and passkeys ([NewAuthService]); first-run setup and the
// bootstrap state machine ([BootstrapService], [NewSetupService]); assets,
// albums, stacks, duplicates, and share links ([AssetService], [AlbumService],
// [StackService], [DuplicateService], [ShareLinkService]); people, places, and
// species ([FaceService], [LocationService], [SpeciesService]); Music
// ([MusicService]); runtime settings ([SettingsService]); indexing and ML
// adapters ([AssetIndexingService], [EmbeddingService], [OCRService],
// [LumenService], [ClassifierService]); and catalog backups ([BackupService]).
//
// Music is a first-class owner-scoped projection over Assets: it reuses Asset
// identity and media delivery but owns track, release, and artist meaning,
// field-level corrections, playlists, and bounded playback snapshots, without
// changing mixed-media Albums.
//
// Services read through catalog readers and write through named catalog
// transactions. They never touch repository paths directly and never run
// filesystem, media, or network work inside a write transaction.
//
// Lumen: the Lumen SDK owns discovery — bounded DNS-SD scans, strict service
// correlation, transport state, and the in-band capability verdict — and the
// Server consumes one immutable SDK runtime snapshot through [LumenService].
// It maps every SDK field it uses from the manifest's lumen section and never
// calls SDK defaults or environment loading. Failed scans keep prior
// observations; only consecutive successful omissions expire a node; mDNS,
// broker, and static sources are additive. GET /api/v1/capabilities exposes
// only aggregate discovery state and task availability, and the
// administrator-only GET /api/v1/admin/lumen/runtime adds bounded per-node
// diagnostics; neither returns raw TXT metadata or resolver errors.
//
// Metadata re-extraction preserves descriptions marked description_edited,
// including an empty user edit; otherwise captions follow the file. See
// preserveSpecificMetadataDescription. On any face re-detection, including a
// model upgrade, captureManualFaceAssignments runs before old faces are
// removed. reapplyManualFaceAssignments matches positive overlaps in descending
// IoU order, with each face and assignment used at most once: IoU >= 0.5 stays
// manual; smaller positive overlap becomes an unconfirmed automatic member.
// Empty people are dissolved only after assignments are reapplied.
//
//atlas:group service
package service
