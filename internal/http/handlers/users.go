package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
)

type UsersHandler struct {
	DB *pgxpool.Pool
}

// GetHostProfile godoc
// @Summary      Get a host's profile
// @Description  Returns the profile information of a specific host, including their ID, name, bio, and avatar URL.
// @Tags         users
// @Produce      json
// @Param        id   path      string  true  "Host User ID"
// @Success      200  {object}  db.HostProfile  "Returns a JSON object containing the host's profile information"
// @Failure      400  {string}  string  "Invalid user ID"
// @Failure      404  {string}  string  "User not found"
// @Failure      500  {string}  string  "Internal Server Error: failed to load profile"
// @Router       /users/{id}/profile [get]
func (h *UsersHandler) GetHostProfile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
 
	hostID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
 
	profile, err := db.GetHostProfile(ctx, h.DB, hostID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load profile", http.StatusInternalServerError)
		return
	}
 
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}