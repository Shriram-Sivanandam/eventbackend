package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
)

// SaveFCMToken godoc
// @Summary      Save a Firebase Cloud Messaging (FCM) token for the authenticated user
// @Description  Saves the provided FCM token for the currently authenticated user. This token can be used to send push notifications to the user's device.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      map[string]string  true  "Request body containing the FCM token"
// @Success      204   {string}  string  "No Content: token saved successfully"
// @Failure      400   {string}  string  "Bad Request: invalid request body or missing token"
// @Failure      401   {string}  string  "Unauthorized: user not authenticated"
// @Failure      500   {string}  string  "Internal Server Error: failed to save token"
// @Router       /auth/fcm-token [post]
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