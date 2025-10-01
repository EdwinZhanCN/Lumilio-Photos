package storage

type DirectoryStructure struct {
	SystemDir  string // .lumilio
	ConfigFile string // .lumiliorepo
	InboxDir   string // inbox

	// System subdirectories
	AssetsDir     string // .lumilio/assets
	ThumbnailsDir string // .lumilio/assets/thumbnails
	VideosDir     string // .lumilio/assets/videos
	AudiosDir     string // .lumilio/assets/audios
	StagingDir    string // .lumilio/staging
	TempDir       string // .lumilio/temp
	TrashDir      string // .lumilio/trash

	// Staging subdirectories
	IncomingDir string // .lumilio/staging/incoming
	FailedDir   string // .lumilio/staging/failed
}

var DefaultStructure = DirectoryStructure{
	SystemDir:     ".lumilio",
	ConfigFile:    ".lumiliorepo",
	InboxDir:      "inbox",
	AssetsDir:     ".lumilio/assets",
	ThumbnailsDir: ".lumilio/assets/thumbnails",
	VideosDir:     ".lumilio/assets/videos",
	AudiosDir:     ".lumilio/assets/audios",
	StagingDir:    ".lumilio/staging",
	TempDir:       ".lumilio/temp",
	TrashDir:      ".lumilio/trash",
	IncomingDir:   ".lumilio/staging/incoming",
	FailedDir:     ".lumilio/staging/failed",
}

var Directories = []string{
	".lumilio",
	".lumilio/assets",
	".lumilio/assets/thumbnails",
	".lumilio/assets/thumbnails/150",
	".lumilio/assets/thumbnails/300",
	".lumilio/assets/thumbnails/1024",
	".lumilio/assets/videos",
	".lumilio/assets/videos/web",
	".lumilio/assets/audios",
	".lumilio/assets/audios/web",
	".lumilio/staging",          // Upload staging area
	".lumilio/staging/incoming", // Upload staging area
	".lumilio/staging/failed",   // Upload staging area
	".lumilio/temp",             // General temporary processing
	".lumilio/trash",            // Soft-deleted user assets
	".lumilio/logs",             // Application and operation logs
	".lumilio/backups",          // Config version backups
	// Content directories
	"inbox", // Structured uploads
}
