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