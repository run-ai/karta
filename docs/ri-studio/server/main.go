package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/run-ai/kai-bolt/docs/ri-studio/server/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Determine paths
	examplesPath := getExamplesPath()
	webDistPath := getWebDistPath()

	log.Printf("Starting RI Studio server on port %s", port)
	log.Printf("Examples path: %s", examplesPath)
	log.Printf("Web dist path: %s", webDistPath)

	// Create API handler
	apiHandler := handlers.NewAPIHandler(examplesPath)

	// Setup routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/validate", corsMiddleware(apiHandler.HandleValidate))
	mux.HandleFunc("/api/extract", corsMiddleware(apiHandler.HandleExtract))
	mux.HandleFunc("/api/examples", corsMiddleware(apiHandler.HandleExamplesList))
	mux.HandleFunc("/api/examples/", corsMiddleware(apiHandler.HandleExample))

	// Serve static files from React build
	if _, err := os.Stat(webDistPath); err == nil {
		fs := http.FileServer(http.Dir(webDistPath))
		mux.Handle("/", http.StripPrefix("/", spaHandler(webDistPath, fs)))
		log.Printf("Serving React app from %s", webDistPath)
	} else {
		log.Printf("Web dist path not found, API-only mode")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("RI Studio API Server - Frontend not built yet. Run 'cd web && npm run build'"))
		})
	}

	// Start server
	log.Printf("Server listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// getExamplesPath returns the path to the examples directory
func getExamplesPath() string {
	// Try environment variable first
	if path := os.Getenv("EXAMPLES_PATH"); path != "" {
		return path
	}

	// Try relative to current directory
	candidates := []string{
		"../examples",
		"docs/examples",
		"../../examples",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			absPath, _ := filepath.Abs(candidate)
			return absPath
		}
	}

	// Default fallback
	return "docs/examples"
}

// getWebDistPath returns the path to the web dist directory
func getWebDistPath() string {
	// Try environment variable first
	if path := os.Getenv("WEB_DIST_PATH"); path != "" {
		return path
	}

	// Try relative to current directory
	candidates := []string{
		"../web/dist",
		"docs/ri-studio/web/dist",
		"web/dist",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			absPath, _ := filepath.Abs(candidate)
			return absPath
		}
	}

	// Default fallback
	return "docs/ri-studio/web/dist"
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin in development
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// spaHandler handles single-page application routing
func spaHandler(staticPath string, fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the file exists
		path := filepath.Join(staticPath, r.URL.Path)
		
		// Check if it's a static file
		if strings.Contains(r.URL.Path, ".") {
			// It has an extension, try to serve it
			if _, err := os.Stat(path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// For all other routes, serve index.html (SPA routing)
		indexPath := filepath.Join(staticPath, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}

		// Fallback
		fileServer.ServeHTTP(w, r)
	}
}

