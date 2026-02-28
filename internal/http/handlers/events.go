package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
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
	City *string `json:"city"`
	AddressLineOne *string `json:"address_line_one"`
	Pincode *string `json:"pincode"`
	MapsLink *string `json:"maps_link"`
	DurationMinutes *int `json:"duration_minutes"`
	ThingsToBring *string `json:"things_to_bring"`
	ThingsProvided *string `json:"things_provided"`
	ImageURL *string `json:"image_url"`
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtrOrNil(s string) *int {
	if s == "" {
		return nil
	}
	v, _ := strconv.Atoi(s)
	return &v
}

func (h *EventsHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)

	defer cancel()

	file, header, err := r.FormFile("image")
	var imageURL *string

	if err == nil {
		defer file.Close()
		filename := uuid.New().String() + filepath.Ext(header.Filename)
		path := "./uploads/" + filename

		out, err := os.Create(path)
		if err != nil {
			http.Error(w, "error in image storage", http.StatusBadRequest)
			return
		}
		defer out.Close()

		io.Copy(out, file)
		url := "/uploads/" + filename
		imageURL = &url
	}

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

	// var req CreateEventRequest
	// if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	// 	http.Error(w, "invalid json body", http.StatusBadRequest)
	// 	return
	// }

	err = r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	location := r.FormValue("location")
	eventStart := r.FormValue("event_start")
	eventEnd := r.FormValue("event_end")
	priceStr := r.FormValue("price")
	city := r.FormValue("city")
	addressLineOne := r.FormValue("address_line_one")
	pincode := r.FormValue("pincode")
	mapsLink := r.FormValue("maps_link")
	durationStr := r.FormValue("duration_minutes")
	thingsToBring := r.FormValue("things_to_bring")
	thingsProvided := r.FormValue("things_provided")

	var duration *int
	if durationStr != "" {
		v, _ := strconv.Atoi(durationStr)
		duration = &v
	}

	price, _ := strconv.Atoi(priceStr)

	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if eventStart == "" {
		http.Error(w, "event_start is required", http.StatusBadRequest)
		return
	}

	start, err := time.Parse(time.RFC3339, eventStart)
	if err != nil {
		http.Error(w, "event_start must be RFC3339", http.StatusBadRequest)
		return
	}

	var end *time.Time
	if eventEnd != "" {
		t, err := time.Parse(time.RFC3339, eventEnd)
		if err != nil {
			http.Error(w, "event_end must be RFC3339", http.StatusBadRequest)
			return
		}
		end = &t
	}

	event := db.Event{
		HostUserID: &uid,
		Title:      title,
		Description: stringPtrOrNil(description),
		Location:   stringPtrOrNil(location),
		EventStart: start,
		EventEnd:   end,
		Price:      price,
		Capacity:   intPtrOrNil(r.FormValue("capacity")),
		City: stringPtrOrNil(city),
		AddressLineOne: stringPtrOrNil(addressLineOne),
		Pincode: stringPtrOrNil(pincode),
		MapsLink: stringPtrOrNil(mapsLink),
		DurationMinutes: duration,
		ThingsToBring: stringPtrOrNil(thingsToBring),
		ThingsProvided: stringPtrOrNil(thingsProvided),
		ImageURL: imageURL,
	}

	created, err := db.CreateEvent(ctx, h.DB, event) 
	if err != nil {
		http.Error(w, "failed to create event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *EventsHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	userIDVal := r.Context().Value(middleware.UserIDKey)
	userID, _ := uuid.Parse(userIDVal.(string))

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

	events, err := db.GetEvents(ctx, h.DB, userID, db.GetEventParams{
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

func (h *EventsHandler) JoinEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	eventIDStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		http.Error(w, "Invalid Event ID", http.StatusBadRequest)
	}

	userIDVal := ctx.Value(middleware.UserIDKey)
	if userIDVal == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userIDStr := userIDVal.(string)
	userID, _ := uuid.Parse(userIDStr)
	err = db.JoinEvent(ctx, h.DB, eventID, userID)
	if err != nil {
		http.Error(w, "Could not join event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}