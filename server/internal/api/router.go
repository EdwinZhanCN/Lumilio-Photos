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

// NewRouter creates and configures a new router with asset, album and auth endpoints
// @title RKPhoto Manager API
// @version 1.0
// @description Photo management system API with asset upload, processing, and organization features
// @host localhost:3001
// @BasePath /api/v1
func NewRouter(assetController AssetControllerInterface, authController AuthControllerInterface, albumController AlbumControllerInterface) *gin.Engine {
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
	}

	return r
}

// corsMiddleware handles CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
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
