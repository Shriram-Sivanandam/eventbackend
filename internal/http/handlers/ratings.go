package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
)

// GET /events/unrated
func (h *EventsHandler) GetUnratedEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	events, err := db.GetUnratedEvents(ctx, h.DB, callerID)
	if err != nil {
		http.Error(w, "failed to fetch unrated events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"events": events,
		"count":  len(events),
	})
}

// POST /events/:id/rate
func (h *EventsHandler) SubmitRating(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		RateeID    string   `json:"ratee_id"`
		RatingType string   `json:"rating_type"`
		Score      int      `json:"score"`
		Comment    *string  `json:"comment"`
		Tags       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	rateeID, err := uuid.Parse(body.RateeID)
	if err != nil {
		http.Error(w, "invalid ratee_id", http.StatusBadRequest)
		return
	}
	if body.RatingType != "host" && body.RatingType != "attendee" {
		http.Error(w, "rating_type must be 'host' or 'attendee'", http.StatusBadRequest)
		return
	}
	if body.Score < 1 || body.Score > 5 {
		http.Error(w, "score must be between 1 and 5", http.StatusBadRequest)
		return
	}
	if callerID == rateeID {
		http.Error(w, "you cannot rate yourself", http.StatusBadRequest)
		return
	}

	var eventStart time.Time
	var hostUserID uuid.UUID
	err = h.DB.QueryRow(ctx,
		`SELECT event_start, host_user_id FROM events WHERE id = $1 AND deleted_at IS NULL`,
		eventID,
	).Scan(&eventStart, &hostUserID)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if time.Now().Before(eventStart) {
		http.Error(w, "cannot rate before the event has started", http.StatusBadRequest)
		return
	}

	switch body.RatingType {
	case "host":
		if rateeID != hostUserID {
			http.Error(w, "ratee_id does not match the event's host", http.StatusBadRequest)
			return
		}

		var isAttendee bool
		err = h.DB.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM event_registrations
				WHERE event_id = $1 AND user_id = $2
				  AND deleted_at IS NULL AND status = 'accepted'
			)
		`, eventID, callerID).Scan(&isAttendee)
		if err != nil || !isAttendee {
			http.Error(w, "you must be a registered attendee to rate the host", http.StatusForbidden)
			return
		}

	case "attendee":
		if callerID != hostUserID {
			http.Error(w, "only the host can rate attendees", http.StatusForbidden)
			return
		}

		var rateeIsAccepted bool
		err = h.DB.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM event_registrations
				WHERE event_id = $1 AND user_id = $2
				  AND deleted_at IS NULL AND status = 'accepted'
			)
		`, eventID, rateeID).Scan(&rateeIsAccepted)
		if err != nil || !rateeIsAccepted {
			http.Error(w, "the attendee must be an accepted registrant to be rated", http.StatusForbidden)
			return
		}
	}

	if err := db.SubmitRating(ctx, h.DB, eventID, callerID, rateeID, body.RatingType, body.Score, body.Comment, body.Tags); err != nil {
		http.Error(w, "failed to submit rating", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /events/{id}/dismiss-rating-prompt
func (h *EventsHandler) DismissRatingPrompt(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
 
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
 
	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
 
	if err := db.DismissRatingPrompt(ctx, h.DB, eventID, callerID); err != nil {
		http.Error(w, "failed to dismiss prompt", http.StatusInternalServerError)
		return
	}
 
	w.WriteHeader(http.StatusNoContent)
}