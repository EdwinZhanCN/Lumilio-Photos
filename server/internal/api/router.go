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
	GetOriginalFile(c *gin.Context)
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
	GetAssetAlbums(c *gin.Context)
}

type PeopleControllerInterface interface {
	ListPeople(c *gin.Context)
	GetPerson(c *gin.Context)
	GetPersonCover(c *gin.Context)
	UpdatePerson(c *gin.Context)
	ListPersonAssets(c *gin.Context)
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

func NewRouter(
	assetController AssetControllerInterface,
	authController AuthControllerInterface,
	albumController AlbumControllerInterface,
	peopleController PeopleControllerInterface,
	queueController QueueControllerInterface,
	statsController StatsControllerInterface,
	agentController AgentControllerInterface,
	capabilitiesController CapabilitiesControllerInterface,
	settingsController SettingsControllerInterface,
	userController UserControllerInterface,
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
			assets.GET("/:id", assetController.GetAsset)
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
			albums.POST("/:id/assets/:assetId", albumController.AddAssetToAlbum)
			albums.DELETE("/:id/assets/:assetId", albumController.RemoveAssetFromAlbum)
			albums.PUT("/:id/assets/:assetId/position", albumController.UpdateAssetPositionInAlbum)
		}

		people := v1.Group("/people")
		people.Use(authController.OptionalAuthMiddleware())
		{
			people.GET("", peopleController.ListPeople)
			people.GET("/:id", peopleController.GetPerson)
			people.GET("/:id/cover", peopleController.GetPersonCover)
			people.PATCH("/:id", authController.AuthMiddleware(), peopleController.UpdatePerson)
			people.POST("/:id/assets/list", peopleController.ListPersonAssets)
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
