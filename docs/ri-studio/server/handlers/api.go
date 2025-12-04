package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/run-ai/kai-bolt/docs/ri-studio/server/models"
	"github.com/run-ai/kai-bolt/docs/ri-studio/server/service"
)

// APIHandler handles all API requests
type APIHandler struct {
	validationService *service.ValidationService
	extractorService  *service.ExtractorService
	examplesPath      string
}

// NewAPIHandler creates a new APIHandler
func NewAPIHandler(examplesPath string) *APIHandler {
	return &APIHandler{
		validationService: service.NewValidationService(),
		extractorService:  service.NewExtractorService(),
		examplesPath:      examplesPath,
	}
}

// HandleValidate handles POST /api/validate
func (h *APIHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	valid, errors := h.validationService.ValidateRI(req.RI)

	response := models.ValidateResponse{
		Valid:  valid,
		Errors: errors,
	}

	h.sendJSON(w, http.StatusOK, response)
}

// HandleExtract handles POST /api/extract
func (h *APIHandler) HandleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.ExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	response, err := h.extractorService.Extract(r.Context(), req.CR, req.RI)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Extraction failed: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, response)
}

// HandleExamplesList handles GET /api/examples
func (h *APIHandler) HandleExamplesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	examples, err := h.listExamples()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list examples: %v", err))
		return
	}

	response := models.ExamplesListResponse{
		Examples: examples,
	}

	h.sendJSON(w, http.StatusOK, response)
}

// HandleExample handles GET /api/examples/:name
func (h *APIHandler) HandleExample(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract name from path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/examples/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		h.sendError(w, http.StatusBadRequest, "Example name is required")
		return
	}
	name := pathParts[0]

	example, err := h.loadExample(name)
	if err != nil {
		if os.IsNotExist(err) {
			h.sendError(w, http.StatusNotFound, fmt.Sprintf("Example '%s' not found", name))
		} else {
			h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load example: %v", err))
		}
		return
	}

	h.sendJSON(w, http.StatusOK, example)
}

// listExamples lists all available examples
func (h *APIHandler) listExamples() ([]models.ExampleInfo, error) {
	files, err := os.ReadDir(h.examplesPath)
	if err != nil {
		return nil, err
	}

	var examples []models.ExampleInfo
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(file.Name(), ".yaml")
		examples = append(examples, models.ExampleInfo{
			Name:        name,
			DisplayName: formatDisplayName(name),
			Description: fmt.Sprintf("Example RI for %s", formatDisplayName(name)),
		})
	}

	return examples, nil
}

// loadExample loads a specific example
func (h *APIHandler) loadExample(name string) (*models.ExampleResponse, error) {
	// Sanitize the name to prevent directory traversal
	name = filepath.Base(name)
	
	riPath := filepath.Join(h.examplesPath, name+".yaml")
	riContent, err := os.ReadFile(riPath)
	if err != nil {
		return nil, err
	}

	return &models.ExampleResponse{
		Name: name,
		RI:   string(riContent),
		// CR is optional - we don't have example CRs, just the RIs
	}, nil
}

// formatDisplayName converts a filename to a display name
func formatDisplayName(name string) string {
	// Convert kebab-case or snake_case to Title Case
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	
	return strings.Join(words, " ")
}

// sendJSON sends a JSON response
func (h *APIHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response
func (h *APIHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, models.ErrorResponse{Error: message})
}




