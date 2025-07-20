package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AssetControllerInterface defines the interface for asset controllers
type AssetControllerInterface interface {
	UploadAsset(c *gin.Context)
	GetAsset(c *gin.Context)
	GetOriginalFile(c *gin.Context)
	ListAssets(c *gin.Context)
	UpdateAsset(c *gin.Context)
	DeleteAsset(c *gin.Context)
	BatchUploadAssets(c *gin.Context)
	AddAssetToAlbum(c *gin.Context)
	GetAssetTypes(c *gin.Context)
	GetThumbnailByID(c *gin.Context)
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

// NewRouter creates and configures a new router with asset and auth endpoints
// @title RKPhoto Manager API
// @version 1.0
// @description Photo management system API with asset upload, processing, and organization features
// @host localhost:3001
// @BasePath /api/v1
func NewRouter(assetController AssetControllerInterface, authController AuthControllerInterface) *gin.Engine {
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
			assets.POST("/batch", assetController.BatchUploadAssets)
			assets.GET("/:id", assetController.GetAsset)
			assets.GET("/:id/original", assetController.GetOriginalFile)
			assets.PUT("/:id", assetController.UpdateAsset)
			assets.DELETE("/:id", assetController.DeleteAsset)
			assets.POST("/:id/albums/:albumId", assetController.AddAssetToAlbum)
		}

		thumbnails := v1.Group("/thumbnails")
		thumbnails.Use(authController.OptionalAuthMiddleware())
		{
			thumbnails.GET("/:id", assetController.GetThumbnailByID)
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
