package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AssetControllerInterface defines the interface for asset controllers
type AssetControllerInterface interface {
	// Legacy asset operations
	UploadAsset(c *gin.Context)
	GetAsset(c *gin.Context)
	GetAssetExif(c *gin.Context)
	GetAssetSidecar(c *gin.Context)
	UpdateAssetSidecar(c *gin.Context)
	GetOriginalFile(c *gin.Context)
	DownloadAssets(c *gin.Context)
	GetWebVideo(c *gin.Context)
	GetWebAudio(c *gin.Context)
	UpdateAsset(c *gin.Context)
	DeleteAsset(c *gin.Context)
	BatchUploadAssets(c *gin.Context)
	GetUploadConfig(c *gin.Context)
	GetUploadProgress(c *gin.Context)
	AddAssetToAlbum(c *gin.Context)
	GetAssetTypes(c *gin.Context)
	GetAssetThumbnail(c *gin.Context)

	// New filtering and search operations
	QueryAssets(c *gin.Context)              // POST /assets/list - Unified asset listing, filtering, and search
	SearchAssets(c *gin.Context)             // POST /assets/search - Sectioned search with top results and fallback results
	ListIndexingRepositories(c *gin.Context) // GET /assets/indexing/repositories - List repositories for indexing filters
	GetIndexingStats(c *gin.Context)         // GET /assets/indexing/stats - Index coverage and queue status
	RebuildAssetIndexes(c *gin.Context)      // POST /assets/indexing/rebuild - Queue reindex backfill for existing assets
	GetFilterOptions(c *gin.Context)         // GET /assets/filter-options - Get available filter options
	GetFeaturedAssets(c *gin.Context)        // GET /assets/featured - Curated featured photos for home/gallery
	GetPhotoMapPoints(c *gin.Context)        // GET /assets/map-points - Lightweight photo map points with GPS

	// Rating management operations
	UpdateAssetRating(c *gin.Context)        // PUT /assets/:id/rating - Update asset rating
	UpdateAssetLike(c *gin.Context)          // PUT /assets/:id/like - Update asset like status
	UpdateAssetRatingAndLike(c *gin.Context) // PUT /assets/:id/rating-and-like - Update both rating and like
	UpdateAssetDescription(c *gin.Context)   // PUT /assets/:id/description - Update asset description
	GetAssetsByRating(c *gin.Context)        // GET /assets/rating/:rating - Get assets by rating
	GetLikedAssets(c *gin.Context)           // GET /assets/liked - Get liked assets

	// Reprocessing operations
	ReprocessAsset(c *gin.Context) // POST /assets/:id/reprocess - Reprocess failed or warning assets

	// Stack operations
	GetAssetStack(c *gin.Context)     // GET /assets/:id/stack - Get stack containing this asset
	CreateManualStack(c *gin.Context) // POST /assets/stacks - Manually create a stack from assets
	UnstackAsset(c *gin.Context)      // DELETE /assets/:id/stack - Remove asset from its stack
	AutoDetectStacks(c *gin.Context)  // POST /repositories/:id/stacks/detect - Auto-detect RAW+JPEG stacks
}

// AuthControllerInterface defines the interface for authentication controllers
type AuthControllerInterface interface {
	StartRegistration(c *gin.Context)
	GetBootstrapStatus(c *gin.Context)
	Login(c *gin.Context)
	BeginRegistrationPasskey(c *gin.Context)
	VerifyRegistrationPasskey(c *gin.Context)
	BeginRegistrationTOTPSetup(c *gin.Context)
	CompleteRegistrationTOTP(c *gin.Context)
	BeginPasskeyLogin(c *gin.Context)
	VerifyPasskeyLogin(c *gin.Context)
	RefreshToken(c *gin.Context)
	Logout(c *gin.Context)
	Me(c *gin.Context)
	GetMediaToken(c *gin.Context)
	VerifyMFA(c *gin.Context)
	GetMFAStatus(c *gin.Context)
	ListPasskeys(c *gin.Context)
	BeginPasskeyEnrollment(c *gin.Context)
	VerifyPasskeyEnrollment(c *gin.Context)
	DeletePasskey(c *gin.Context)
	BeginTOTPSetup(c *gin.Context)
	EnableTOTP(c *gin.Context)
	DisableTOTP(c *gin.Context)
	RegenerateRecoveryCodes(c *gin.Context)
	AuthMiddleware() gin.HandlerFunc
	OptionalAuthMiddleware() gin.HandlerFunc
	RequireAdmin() gin.HandlerFunc
}

// AlbumControllerInterface defines the interface for album controllers
type AlbumControllerInterface interface {
	NewAlbum(c *gin.Context)
	GetAlbum(c *gin.Context)
	ListAlbums(c *gin.Context)
	UpdateAlbum(c *gin.Context)
	DeleteAlbum(c *gin.Context)
	GetAlbumAssets(c *gin.Context)
	AddAssetToAlbum(c *gin.Context)
	RemoveAssetFromAlbum(c *gin.Context)
	UpdateAssetPositionInAlbum(c *gin.Context)
	RebuildAlbumBioClip(c *gin.Context)
	GetAssetAlbums(c *gin.Context)
}

type PeopleControllerInterface interface {
	ListPeople(c *gin.Context)
	RebuildPeople(c *gin.Context)
	GetPerson(c *gin.Context)
	GetPersonCover(c *gin.Context)
	UpdatePerson(c *gin.Context)
	ListPersonAssets(c *gin.Context)
}

type LocationControllerInterface interface {
	ListLocationClusters(c *gin.Context)
	RebuildLocationClusters(c *gin.Context)
}

type SpeciesControllerInterface interface {
	GetSpeciesReference(c *gin.Context)
}

// QueueControllerInterface defines the interface for queue monitoring controllers
type QueueControllerInterface interface {
	ListJobs(c *gin.Context)
	GetJob(c *gin.Context)
	ListQueues(c *gin.Context)
	GetJobStats(c *gin.Context)
}

// StatsControllerInterface defines the interface for statistics controllers
type StatsControllerInterface interface {
	GetFocalLengthDistribution(c *gin.Context) // GET /stats/focal-length - Get focal length distribution
	GetCameraLensStats(c *gin.Context)         // GET /stats/camera-lens - Get camera+lens combination stats
	GetTimeDistribution(c *gin.Context)        // GET /stats/time-distribution - Get time distribution
	GetDailyActivityHeatmap(c *gin.Context)    // GET /stats/daily-activity - Get daily activity heatmap
	GetAvailableYears(c *gin.Context)          // GET /stats/available-years - Get available years
}

// AgentControllerInterface defines the interface for agent controllers
type AgentControllerInterface interface {
	Chat(c *gin.Context)           // POST /agent/chat - Chat with agent via SSE
	ResumeChat(c *gin.Context)     // POST /agent/chat/resume - Resume an interrupted agent execution
	GetTools(c *gin.Context)       // GET /agent/tools - Get available tools
	GetToolSchemas(c *gin.Context) // GET /agent/schemas - Get tool DTO schemas
}

// CapabilitiesControllerInterface defines the interface for public system capability controllers.
type CapabilitiesControllerInterface interface {
	GetCapabilities(c *gin.Context) // GET /capabilities - Get de-sensitized runtime capabilities
}

type SettingsControllerInterface interface {
	GetSystemSettings(c *gin.Context)
	UpdateSystemSettings(c *gin.Context)
	ValidateLLMSettings(c *gin.Context)
}

type UserControllerInterface interface {
	UpdateMyProfile(c *gin.Context)
	ChangeMyPassword(c *gin.Context)
	ListUsers(c *gin.Context)
	UpdateUser(c *gin.Context)
	ResetUserAccess(c *gin.Context)
}

type RepositoryScanControllerInterface interface {
	CreateRepository(c *gin.Context)
	ListRepositories(c *gin.Context)
	GetRepository(c *gin.Context)
	UpdateRepository(c *gin.Context)
	DeleteRepository(c *gin.Context)
	QueueRepositoryScan(c *gin.Context)
	GetLatestRepositoryScan(c *gin.Context)
	ListRepositoryScans(c *gin.Context)
}

// DuplicateControllerInterface defines the Utilities Rail "Duplicates" endpoints.
type DuplicateControllerInterface interface {
	GetDuplicateSummary(c *gin.Context)   // GET    /duplicates/summary
	ListDuplicateGroups(c *gin.Context)   // GET    /duplicates/groups
	GetDuplicateGroup(c *gin.Context)     // GET    /duplicates/groups/:id
	DetectDuplicates(c *gin.Context)      // POST   /duplicates/detect
	MergeDuplicateGroup(c *gin.Context)   // POST   /duplicates/groups/:id/merge
	DismissDuplicateGroup(c *gin.Context) // POST   /duplicates/groups/:id/dismiss
}

// CloudControllerInterface defines the cloud sync endpoints.
type CloudControllerInterface interface {
	ListCredentials(c *gin.Context)           // GET    /cloud/credentials
	CreateICloudCredential(c *gin.Context)    // POST   /cloud/icloud/credentials
	VerifyICloudCredential2FA(c *gin.Context) // POST /cloud/icloud/credentials/:id/verify-2fa
	DisableCredential(c *gin.Context)         // DELETE /cloud/credentials/:id
	StartRepositoryImport(c *gin.Context)     // POST   /repositories/:id/cloud/import
	GetRepositoryCloudStatus(c *gin.Context)  // GET   /repositories/:id/cloud
	GetImportRun(c *gin.Context)              // GET    /cloud/import-runs/:id
	TriggerSync(c *gin.Context)               // POST   /cloud/sync (deprecated)
}

// SetupControllerInterface defines the zero-config first-run setup endpoints.
type SetupControllerInterface interface {
	GetSetupStatus(c *gin.Context) // GET  /setup/status
	Setup(c *gin.Context)          // POST /setup
}

func NewRouter(
	assetController AssetControllerInterface,
	authController AuthControllerInterface,
	setupController SetupControllerInterface,
	albumController AlbumControllerInterface,
	peopleController PeopleControllerInterface,
	locationController LocationControllerInterface,
	speciesController SpeciesControllerInterface,
	queueController QueueControllerInterface,
	statsController StatsControllerInterface,
	agentController AgentControllerInterface,
	capabilitiesController CapabilitiesControllerInterface,
	settingsController SettingsControllerInterface,
	userController UserControllerInterface,
	repositoryScanController RepositoryScanControllerInterface,
	duplicateController DuplicateControllerInterface,
	cloudController CloudControllerInterface,
	agentAvailabilityMiddleware gin.HandlerFunc,
) *gin.Engine {
	r := gin.Default()
	allowedOrigins := loadAllowedCORSOrigins()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		corsMiddleware(allowedOrigins, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	})

	// API routes
	api := r.Group("/api")

	// V1 API routes
	v1 := api.Group("/v1")
	{
		// Health check
		v1.GET("/health", func(c *gin.Context) {
			GinSuccess(c, gin.H{"status": "ok"})
		})
		v1.GET("/capabilities", authController.OptionalAuthMiddleware(), capabilitiesController.GetCapabilities)

		// Zero-config first-run setup. Public: the system has no users/secrets yet.
		setup := v1.Group("/setup")
		{
			setup.GET("/status", setupController.GetSetupStatus)
			setup.POST("", setupController.Setup)
		}

		settings := v1.Group("/settings")
		settings.Use(authController.AuthMiddleware(), authController.RequireAdmin())
		{
			settings.GET("/system", settingsController.GetSystemSettings)
			settings.PATCH("/system", settingsController.UpdateSystemSettings)
			settings.POST("/system/validate-llm", settingsController.ValidateLLMSettings)
		}

		// Authentication routes
		auth := v1.Group("/auth")
		{
			auth.GET("/bootstrap-status", authController.GetBootstrapStatus)
			auth.POST("/register/start", authController.StartRegistration)
			auth.POST("/register/totp/setup", authController.BeginRegistrationTOTPSetup)
			auth.POST("/register/totp/complete", authController.CompleteRegistrationTOTP)
			auth.POST("/login", authController.Login)
			auth.POST("/passkeys/register/options", authController.BeginRegistrationPasskey)
			auth.POST("/passkeys/register/verify", authController.VerifyRegistrationPasskey)
			auth.POST("/passkeys/login/options", authController.BeginPasskeyLogin)
			auth.POST("/passkeys/login/verify", authController.VerifyPasskeyLogin)
			auth.POST("/mfa/verify", authController.VerifyMFA)
			auth.POST("/refresh", authController.RefreshToken)
			auth.POST("/logout", authController.Logout)
			auth.GET("/me", authController.AuthMiddleware(), authController.Me)
			auth.GET("/media-token", authController.AuthMiddleware(), authController.GetMediaToken)
			auth.GET("/mfa", authController.AuthMiddleware(), authController.GetMFAStatus)
			auth.GET("/mfa/passkeys", authController.AuthMiddleware(), authController.ListPasskeys)
			auth.POST("/mfa/passkeys/options", authController.AuthMiddleware(), authController.BeginPasskeyEnrollment)
			auth.POST("/mfa/passkeys/verify", authController.AuthMiddleware(), authController.VerifyPasskeyEnrollment)
			auth.DELETE("/mfa/passkeys/:id", authController.AuthMiddleware(), authController.DeletePasskey)
			auth.POST("/mfa/totp/setup", authController.AuthMiddleware(), authController.BeginTOTPSetup)
			auth.POST("/mfa/totp/enable", authController.AuthMiddleware(), authController.EnableTOTP)
			auth.POST("/mfa/totp/disable", authController.AuthMiddleware(), authController.DisableTOTP)
			auth.POST("/mfa/recovery-codes/regenerate", authController.AuthMiddleware(), authController.RegenerateRecoveryCodes)
		}

		users := v1.Group("/users")
		users.Use(authController.AuthMiddleware())
		{
			users.PATCH("/me/profile", userController.UpdateMyProfile)
			users.PATCH("/me/password", userController.ChangeMyPassword)
			users.GET("", authController.RequireAdmin(), userController.ListUsers)
			users.PATCH("/:id", authController.RequireAdmin(), userController.UpdateUser)
			users.POST("/:id/reset-access", authController.RequireAdmin(), userController.ResetUserAccess)
		}

		repositories := v1.Group("/repositories")
		repositories.Use(authController.AuthMiddleware(), authController.RequireAdmin())
		{
			repositories.GET("", repositoryScanController.ListRepositories)
			repositories.POST("", repositoryScanController.CreateRepository)
			repositories.GET("/:id", repositoryScanController.GetRepository)
			repositories.PATCH("/:id", repositoryScanController.UpdateRepository)
			repositories.DELETE("/:id", repositoryScanController.DeleteRepository)
			repositories.GET("/:id/cloud", cloudController.GetRepositoryCloudStatus)
			repositories.POST("/:id/cloud/import", cloudController.StartRepositoryImport)
			repositories.POST("/:id/scan", repositoryScanController.QueueRepositoryScan)
			repositories.GET("/:id/scans/latest", repositoryScanController.GetLatestRepositoryScan)
			repositories.GET("/:id/scans", repositoryScanController.ListRepositoryScans)
			repositories.POST("/:id/stacks/detect", assetController.AutoDetectStacks)
		}

		locations := v1.Group("/locations")
		locations.Use(authController.OptionalAuthMiddleware())
		{
			locations.GET("/clusters", locationController.ListLocationClusters)
			locations.POST("/rebuild", authController.AuthMiddleware(), authController.RequireAdmin(), locationController.RebuildLocationClusters)
		}

		species := v1.Group("/species")
		species.Use(authController.OptionalAuthMiddleware())
		{
			species.GET("/reference", speciesController.GetSpeciesReference)
		}

		// Asset routes (new unified API) - with optional authentication
		assets := v1.Group("/assets")
		assets.Use(authController.OptionalAuthMiddleware())
		{
			assets.POST("", assetController.UploadAsset)
			assets.GET("/types", assetController.GetAssetTypes)
			assets.GET("/filter-options", assetController.GetFilterOptions)
			assets.GET("/featured", assetController.GetFeaturedAssets)
			assets.GET("/map-points", assetController.GetPhotoMapPoints)
			assets.GET("/indexing/repositories", authController.AuthMiddleware(), authController.RequireAdmin(), assetController.ListIndexingRepositories)
			assets.GET("/indexing/stats", authController.AuthMiddleware(), authController.RequireAdmin(), assetController.GetIndexingStats)
			assets.POST("/indexing/rebuild", authController.AuthMiddleware(), authController.RequireAdmin(), assetController.RebuildAssetIndexes)
			assets.POST("/list", assetController.QueryAssets)
			assets.POST("/search", assetController.SearchAssets)
			assets.POST("/batch", assetController.BatchUploadAssets)
			assets.GET("/batch/config", assetController.GetUploadConfig)
			assets.GET("/batch/progress", assetController.GetUploadProgress)
			assets.POST("/download", assetController.DownloadAssets)
			assets.GET("/:id", assetController.GetAsset)
			assets.GET("/:id/exif", assetController.GetAssetExif)
			assets.GET("/:id/sidecar", assetController.GetAssetSidecar)
			assets.PUT("/:id/sidecar", assetController.UpdateAssetSidecar)
			assets.GET("/:id/original", assetController.GetOriginalFile)
			assets.HEAD("/:id/original", assetController.GetOriginalFile)
			assets.GET("/:id/video/web", assetController.GetWebVideo)
			assets.HEAD("/:id/video/web", assetController.GetWebVideo)
			assets.GET("/:id/audio/web", assetController.GetWebAudio)
			assets.HEAD("/:id/audio/web", assetController.GetWebAudio)
			assets.GET("/:id/thumbnail", assetController.GetAssetThumbnail)
			assets.HEAD("/:id/thumbnail", assetController.GetAssetThumbnail)
			assets.PUT("/:id", assetController.UpdateAsset)
			assets.DELETE("/:id", assetController.DeleteAsset)
			assets.POST("/:id/albums/:albumId", assetController.AddAssetToAlbum)
			assets.GET("/:id/albums", albumController.GetAssetAlbums)

			// Rating management routes
			assets.PUT("/:id/rating", assetController.UpdateAssetRating)
			assets.PUT("/:id/like", assetController.UpdateAssetLike)
			assets.PUT("/:id/rating-and-like", assetController.UpdateAssetRatingAndLike)
			assets.PUT("/:id/description", assetController.UpdateAssetDescription)
			assets.GET("/rating/:rating", assetController.GetAssetsByRating)
			assets.GET("/liked", assetController.GetLikedAssets)
			assets.POST("/:id/reprocess", assetController.ReprocessAsset)

			// Stack routes
			assets.GET("/:id/stack", assetController.GetAssetStack)
			assets.DELETE("/:id/stack", assetController.UnstackAsset)
			assets.POST("/stacks", assetController.CreateManualStack)
		}

		// Album routes - with authentication required
		albums := v1.Group("/albums")
		albums.Use(authController.AuthMiddleware())
		{
			albums.POST("", albumController.NewAlbum)
			albums.GET("", albumController.ListAlbums)
			albums.GET("/:id", albumController.GetAlbum)
			albums.PUT("/:id", albumController.UpdateAlbum)
			albums.DELETE("/:id", albumController.DeleteAlbum)
			albums.GET("/:id/assets", albumController.GetAlbumAssets)
			albums.POST("/:id/bioclip/rebuild", albumController.RebuildAlbumBioClip)
			albums.POST("/:id/assets/:assetId", albumController.AddAssetToAlbum)
			albums.DELETE("/:id/assets/:assetId", albumController.RemoveAssetFromAlbum)
			albums.PUT("/:id/assets/:assetId/position", albumController.UpdateAssetPositionInAlbum)
		}

		people := v1.Group("/people")
		people.Use(authController.OptionalAuthMiddleware())
		{
			people.GET("", peopleController.ListPeople)
			people.POST("/rebuild", authController.AuthMiddleware(), peopleController.RebuildPeople)
			people.GET("/:id", peopleController.GetPerson)
			people.GET("/:id/cover", peopleController.GetPersonCover)
			people.PATCH("/:id", authController.AuthMiddleware(), peopleController.UpdatePerson)
			people.POST("/:id/assets/list", peopleController.ListPersonAssets)
		}

		// Duplicate detection routes (Utilities Rail). Auth is required for
		// mutating actions (detect/merge/dismiss); reads are open to authed users.
		duplicates := v1.Group("/duplicates")
		duplicates.Use(authController.AuthMiddleware())
		{
			duplicates.GET("/summary", duplicateController.GetDuplicateSummary)
			duplicates.GET("/groups", duplicateController.ListDuplicateGroups)
			duplicates.GET("/groups/:id", duplicateController.GetDuplicateGroup)
			duplicates.POST("/detect", duplicateController.DetectDuplicates)
			duplicates.POST("/groups/:id/merge", duplicateController.MergeDuplicateGroup)
			duplicates.POST("/groups/:id/dismiss", duplicateController.DismissDuplicateGroup)
		}

		// Cloud sync routes - admin only
		cloud := v1.Group("/cloud")
		cloud.Use(authController.AuthMiddleware(), authController.RequireAdmin())
		{
			cloud.GET("/credentials", cloudController.ListCredentials)
			cloud.POST("/icloud/credentials", cloudController.CreateICloudCredential)
			cloud.POST("/icloud/credentials/:id/verify-2fa", cloudController.VerifyICloudCredential2FA)
			cloud.DELETE("/credentials/:id", cloudController.DisableCredential)
			cloud.GET("/import-runs/:id", cloudController.GetImportRun)
			cloud.POST("/sync", cloudController.TriggerSync)
		}

		// Admin routes for queue monitoring (read-only)
		admin := v1.Group("/admin")
		admin.Use(authController.AuthMiddleware(), authController.RequireAdmin())
		{
			river := admin.Group("/river")
			{
				river.GET("/jobs", queueController.ListJobs)
				river.GET("/jobs/:id", queueController.GetJob)
				river.GET("/queues", queueController.ListQueues)
				river.GET("/stats", queueController.GetJobStats)
			}
		}

		// Stats routes - with optional authentication
		stats := v1.Group("/stats")
		stats.Use(authController.OptionalAuthMiddleware())
		{
			stats.GET("/focal-length", statsController.GetFocalLengthDistribution)
			stats.GET("/camera-lens", statsController.GetCameraLensStats)
			stats.GET("/time-distribution", statsController.GetTimeDistribution)
			stats.GET("/daily-activity", statsController.GetDailyActivityHeatmap)
			stats.GET("/available-years", statsController.GetAvailableYears)
		}

		// Agent routes - with optional authentication
		agent := v1.Group("/agent")
		agent.Use(agentAvailabilityMiddleware, authController.OptionalAuthMiddleware())
		{
			agent.POST("/chat", agentController.Chat)
			agent.POST("/chat/resume", agentController.ResumeChat)
			agent.GET("/tools", agentController.GetTools)
			agent.GET("/schemas", agentController.GetToolSchemas)
		}
	}

	return r
}

func loadAllowedCORSOrigins() map[string]struct{} {
	origins := map[string]struct{}{
		"http://localhost:6657":  {},
		"https://localhost:6657": {},
	}

	rawOrigins := strings.TrimSpace(os.Getenv("SERVER_CORS_ALLOWED_ORIGINS"))
	if rawOrigins == "" {
		return origins
	}

	customOrigins := make(map[string]struct{})
	for _, origin := range strings.Split(rawOrigins, ",") {
		normalized := strings.TrimSpace(origin)
		if normalized != "" {
			customOrigins[normalized] = struct{}{}
		}
	}

	if len(customOrigins) == 0 {
		return origins
	}

	return customOrigins
}

// corsMiddleware handles CORS headers
func corsMiddleware(allowedOrigins map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-content-hash")
		w.Header().Set("Vary", "Origin")

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if _, allowed := allowedOrigins[origin]; allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		if r.Method == "OPTIONS" {
			if origin != "" {
				if _, allowed := allowedOrigins[origin]; !allowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
