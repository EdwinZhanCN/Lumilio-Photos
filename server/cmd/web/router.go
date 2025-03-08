package web

import (
	"github.com/gorilla/mux"
	"net/http"
)

// PhotoControllerInterface defines the interface for photo controllers
type PhotoControllerInterface interface {
	UploadPhoto(w http.ResponseWriter, r *http.Request)
}

// NewRouter creates and configures a new router
func NewRouter(photoController PhotoControllerInterface) *mux.Router {
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api").Subrouter()

	// Photo routes
	photoRoutes := api.PathPrefix("/photos").Subrouter()
	photoRoutes.HandleFunc("", photoController.UploadPhoto).Methods("POST")

	// Add CORS middleware
	r.Use(corsMiddleware)

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