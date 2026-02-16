package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
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

	userIDVal := r.Context().Value(middleware.UserIDKey)
	if userIDVal == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, ok := userIDVal.(string)
	if !ok {
		http.Error(w, "invalid auth context", http.StatusUnauthorized)
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

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

	// var hostUserID *uuid.UUID
	// if req.HostUserID != nil {
	// 	id, err := uuid.Parse(*req.HostUserID)

	// 	if err != nil {
	// 		http.Error(w, "invalid user id", http.StatusBadRequest)
	// 		return
	// 	}
	// 	hostUserID = &id
	// }

	// var hostPageID *uuid.UUID
	// if req.HostPageID != nil {
	// 	id, err := uuid.Parse(*req.HostPageID)
	// 	if err != nil {
	// 		http.Error(w, "invalid host_page_id", http.StatusBadRequest)
	// 		return
	// 	}
	// 	hostPageID = &id
	// }

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
		HostUserID: &uid,
		Title:      req.Title,
		Description: req.Description,
		Location:   req.Location,
		EventStart: start,
		EventEnd:   end,
		Price:      price,
		Capacity:   req.Capacity,
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

func (h *EventsHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	q := r.URL.Query()

	var hostUserID *uuid.UUID
	if v := q.Get("host_user_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			http.Error(w, "invalid host_user_id", http.StatusBadRequest)
			return
		}
		hostUserID = &id
	}

	var pageID *uuid.UUID
	if v := q.Get("page_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			http.Error(w, "invalid page_id", http.StatusBadRequest)
			return
		}
		pageID = &id
	}

	var from *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "invalid from (use RFC3339)", http.StatusBadRequest)
			return
		}
		from = &t
	}

	var to *time.Time
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "invalid to (use RFC3339)", http.StatusBadRequest)
			return
		}
		to = &t
	}

	limit := 20
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = n
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
		offset = n
	}

	events, err := db.GetEvents(ctx, h.DB, db.GetEventParams{
		HostUserID: hostUserID,
		PageID: pageID,
		From: from,
		To: to,
		Limit: limit,
		Offset: offset,
	})
	if err != nil {
		http.Error(w, "failed to fetch events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"events": events,
		"count": len(events),
	})
}