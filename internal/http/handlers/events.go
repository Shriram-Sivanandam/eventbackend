package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
)

type EventsHandler struct {
	DB *pgxpool.Pool
}

type CreateEventRequest struct {
	HostUserID *string `json:"host_user_id"`
	HostPageID *string `json:"host_page_id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Location    *string `json:"location"`
	EventStart string  `json:"event_start"`
	EventEnd   *string `json:"event_end"`
	Price    *int `json:"price"`
	Capacity *int `json:"capacity"`
}

func (h *EventsHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)

	defer cancel()

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if req.EventStart == "" {
		http.Error(w, "event_start is required", http.StatusBadRequest)
		return
	}
	if req.HostUserID == nil && req.HostPageID == nil {
		http.Error(w, "host_user_id or host_page_id is required", http.StatusBadRequest)
		return
	}

	var hostUserID *uuid.UUID
	if req.HostUserID != nil {
		id, err := uuid.Parse(*req.HostUserID)

		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}
		hostUserID = &id
	}

	var hostPageID *uuid.UUID
	if req.HostPageID != nil {
		id, err := uuid.Parse(*req.HostPageID)
		if err != nil {
			http.Error(w, "invalid host_page_id", http.StatusBadRequest)
			return
		}
		hostPageID = &id
	}

	start, err := time.Parse(time.RFC3339, req.EventStart)
	if err != nil {
		http.Error(w, "event_start must be RFC3339", http.StatusBadRequest)
		return
	}

	var end *time.Time
	if req.EventEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.EventEnd)
		if err != nil {
			http.Error(w, "event_end must be RFC3339", http.StatusBadRequest)
			return
		}
		end = &t
	}

	price := 0
	if req.Price != nil {
		price = *req.Price
	}

	event := db.Event{
		HostUserID:  hostUserID,
		HostPageID:  hostPageID,
		Title:       req.Title,
		Description: req.Description,
		Location:    req.Location,
		EventStart:  start,
		EventEnd:    end,
		Price:       price,
		Capacity:    req.Capacity,
	}

	created, err := db.CreateEvent(ctx, h.DB, event) 
	if err != nil {
		http.Error(w, "failed to create event", http. StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}