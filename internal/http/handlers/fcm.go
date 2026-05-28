package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
)

// POST /auth/fcm-token
func (h *AuthHandler) SaveFCMToken(w http.ResponseWriter, r *http.Request) {
	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if err := db.SaveFCMToken(r.Context(), h.DB, callerID, body.Token); err != nil {
		http.Error(w, "failed to save token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}