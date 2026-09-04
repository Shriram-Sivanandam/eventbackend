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

// GetUnratedEvents godoc
// @Summary      Get unrated events for the authenticated user
// @Description  Returns a list of events that the currently authenticated user has attended but not yet rated. This allows users to provide feedback on events they have participated in.
// @Tags         ratings
// @Produce      json
// @Success      200  {object}  map[string][]db.Event  "Returns a JSON object containing a list of unrated events"
// @Failure      401  {string}  string  "Unauthorized"
// @Failure      500  {string}  string  "Internal Server Error: failed to fetch unrated events"
// @Router       /events/unrated [get]
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

// SubmitRating godoc
// @Summary      Submit a rating for an event
// @Description  Allows the currently authenticated user to submit a rating for another user (either the host or an attendee) of a specific event. The rating includes a score, optional comment, and optional tags.
// @Tags         ratings
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Event ID"
// @Param        body  body      map[string]interface{}  true  "Request body containing ratee_id, rating_type, score, comment, and tags"
// @Success      204   {string}  string  "No Content: rating submitted successfully"
// @Failure      400   {string}  string  "Bad Request: invalid input or constraints not met"
// @Failure      401   {string}  string  "Unauthorized: user not authenticated"
// @Failure      403   {string}  string  "Forbidden: user not allowed to rate the specified user"
// @Failure      404   {string}  string  "Not Found: event not found"
// @Failure      500   {string}  string  "Internal Server Error: failed to submit rating"
// @Router       /events/{id}/rate [post]
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

// DismissRatingPrompt godoc
// @Summary      Dismiss the rating prompt for an event
// @Description  Allows the currently authenticated user to dismiss the rating prompt for a specific event. This indicates that the user does not wish to provide a rating for that event at this time.
// @Tags         ratings
// @Produce      json
// @Param        id    path      string  true  "Event ID"
// @Success      204   {string}  string  "No Content: rating prompt dismissed successfully"
// @Failure      400   {string}  string  "Bad Request: invalid event ID"
// @Failure      401   {string}  string  "Unauthorized: user not authenticated"
// @Failure      500   {string}  string  "Internal Server Error: failed to dismiss rating prompt"
// @Router       /events/{id}/dismiss-rating-prompt [post]
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