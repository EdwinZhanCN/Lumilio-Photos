package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AssetControllerInterface defines the interface for asset controllers
type AssetControllerInterface interface {
	UploadAsset(c *gin.Context)
	GetAsset(c *gin.Context)
	ListAssets(c *gin.Context)
	UpdateAsset(c *gin.Context)
	DeleteAsset(c *gin.Context)
	BatchUploadAssets(c *gin.Context)
	AddAssetToAlbum(c *gin.Context)
	GetAssetTypes(c *gin.Context)
}

// NewRouter creates and configures a new router with asset endpoints
// @title RKPhoto Manager API
// @version 1.0
// @description Photo management system API with asset upload, processing, and organization features
// @host localhost:3001
// @BasePath /api/v1
func NewRouter(assetController AssetControllerInterface) *gin.Engine {
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	})

	// API routes
	api := r.Group("/api")

	// Asset routes (new unified API)
	v1 := api.Group("/v1")
	{
		assets := v1.Group("/assets")
		{
			assets.POST("", assetController.UploadAsset)
			assets.GET("", assetController.ListAssets)
			assets.GET("/types", assetController.GetAssetTypes)
			assets.POST("/batch", assetController.BatchUploadAssets)
			assets.GET("/:id", assetController.GetAsset)
			assets.PUT("/:id", assetController.UpdateAsset)
			assets.DELETE("/:id", assetController.DeleteAsset)
			assets.POST("/:id/albums/:albumId", assetController.AddAssetToAlbum)
		}
	}

	return r
}

// corsMiddleware handles CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add frontend URL from config
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
