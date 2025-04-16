package web

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PhotoControllerInterface defines the interface for photo controllers
type PhotoControllerInterface interface {
	UploadPhoto(w http.ResponseWriter, r *http.Request)
	BatchUploadPhotos(w http.ResponseWriter, r *http.Request)
}

// NewRouter creates and configures a new router
func NewRouter(photoController PhotoControllerInterface) *gin.Engine {
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	})

	// API routes
	api := r.Group("/api")

	// Photo routes
	photoRoutes := api.Group("/photos")

	photoRoutes.POST("", func(c *gin.Context) {
		photoController.UploadPhoto(c.Writer, c.Request)
	})

	photoRoutes.POST("/batch", func(c *gin.Context) {
		photoController.BatchUploadPhotos(c.Writer, c.Request)
	})

	log.Println("Starting Controller")

	return r
}

// corsMiddleware handles CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
