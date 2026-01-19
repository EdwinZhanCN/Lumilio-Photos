package api

import (
	"net/http"

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
	ListAssets(c *gin.Context)
	UpdateAsset(c *gin.Context)
	DeleteAsset(c *gin.Context)
	BatchUploadAssets(c *gin.Context)
	AddAssetToAlbum(c *gin.Context)
	GetAssetTypes(c *gin.Context)
	GetAssetThumbnail(c *gin.Context)

	// New filtering and search operations
	FilterAssets(c *gin.Context)     // POST /assets/filter - Complex asset filtering
	SearchAssets(c *gin.Context)     // POST /assets/search - Filename and semantic search
	GetFilterOptions(c *gin.Context) // GET /assets/filter-options - Get available filter options

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
	Register(c *gin.Context)
	Login(c *gin.Context)
	RefreshToken(c *gin.Context)
	Logout(c *gin.Context)
	Me(c *gin.Context)
	AuthMiddleware() gin.HandlerFunc
	OptionalAuthMiddleware() gin.HandlerFunc
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
	GetTools(c *gin.Context)       // GET /agent/tools - Get available tools
	GetToolSchemas(c *gin.Context) // GET /agent/schemas - Get tool DTO schemas
}

func NewRouter(assetController AssetControllerInterface, authController AuthControllerInterface, albumController AlbumControllerInterface, queueController QueueControllerInterface, statsController StatsControllerInterface, agentController AgentControllerInterface) *gin.Engine {
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		// Authentication routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authController.Register)
			auth.POST("/login", authController.Login)
			auth.POST("/refresh", authController.RefreshToken)
			auth.POST("/logout", authController.Logout)
			auth.GET("/me", authController.AuthMiddleware(), authController.Me)
		}

		// Asset routes (new unified API) - with optional authentication
		assets := v1.Group("/assets")
		assets.Use(authController.OptionalAuthMiddleware())
		{
			assets.POST("", assetController.UploadAsset)
			assets.GET("", assetController.ListAssets)
			assets.GET("/types", assetController.GetAssetTypes)
			assets.GET("/filter-options", assetController.GetFilterOptions)
			assets.POST("/filter", assetController.FilterAssets)
			assets.POST("/search", assetController.SearchAssets)
			assets.POST("/batch", assetController.BatchUploadAssets)
			assets.GET("/:id", assetController.GetAsset)
			assets.GET("/:id/original", assetController.GetOriginalFile)
			assets.GET("/:id/video/web", assetController.GetWebVideo)
			assets.GET("/:id/audio/web", assetController.GetWebAudio)
			assets.GET("/:id/thumbnail", assetController.GetAssetThumbnail)
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

		// Admin routes for queue monitoring (read-only)
		admin := v1.Group("/admin")
		admin.Use(authController.OptionalAuthMiddleware())
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
		agent.Use(authController.OptionalAuthMiddleware())
		{
			agent.POST("/chat", agentController.Chat)
			agent.GET("/tools", agentController.GetTools)
			agent.GET("/schemas", agentController.GetToolSchemas)
		}
	}

	return r
}

// corsMiddleware handles CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:6657")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
