package handler

import (
	"encoding/json"
	"fundtracking/internal/service"
	"net/http"
)

type HealthHandler struct {
	healthService *service.HealthService
}

func NewHealthHandler(healthService *service.HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}
func (h *HealthHandler) APIVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	payload := h.healthService.GetVersion()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
