package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
	"github.com/Shriram-Sivanandam/eventbackend/internal/notify"
	"github.com/Shriram-Sivanandam/eventbackend/internal/storage"
)

type EventsHandler struct {
	DB *pgxpool.Pool
	R2 *storage.R2Client
	FirebaseAuth *auth.Client
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

// CreateEvent godoc
// @Summary      Create a new event
// @Description  Creates a new event with the provided details. The authenticated user will be set as the host of the event.
// @Tags         events
// @Accept       multipart/form-data
// @Produce      json
// @Param        title              formData  string  true   "Event title"
// @Param        description        formData  string  false  "Event description"
// @Param        location           formData  string  false  "Event location"
// @Param        event_start        formData  string  true   "Event start time (RFC3339 format)"
// @Param        event_end          formData  string  false  "Event end time (RFC3339 format)"
// @Param        price              formData  int     false  "Event price"
// @Param        capacity           formData  int     false  "Event capacity"
// @Param        city               formData  string  false  "City where the event is held"
// @Param        address_line_one   formData  string  false  "Address line one"
// @Param        pincode            formData  string  false  "Pincode"
// @Param        maps_link          formData  string  false  "Google Maps link"
// @Param        duration_minutes   formData  int     false  "Event duration in minutes"
// @Param        things_to_bring    formData  string  false  "Things attendees should bring"
// @Param        things_provided    formData  string  false  "Things provided by the host"
// @Param        image              formData  file    false  "Event image file"
// @Param        tag_ids            formData  []string false "List of tag IDs associated with the event"
// @Success      201                {object}  db.Event
// @Failure      400                {string}  string  "Bad Request: invalid input data"
// @Failure      401                {string}  string  "Unauthorized: user not authenticated"
// @Failure      500                {string}  string  "Internal Server Error: failed to create event"
// @Router       /events [post]
func (h *EventsHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)

	defer cancel()

	file, header, err := r.FormFile("image")
	var imageURL *string

	if err == nil {
		defer file.Close()
	
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "failed to read image", http.StatusInternalServerError)
			return
		}
	
		ext := filepath.Ext(header.Filename)
		key := "events/" + uuid.New().String() + ext
	
		url, err := h.R2.Upload(r.Context(), key, data, ext)
		if err != nil {
			http.Error(w, "failed to upload image", http.StatusInternalServerError)
			return
		}
	
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

	err = r.ParseMultipartForm(10 << 20)
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
	tagIDs := r.Form["tag_ids"]

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

	for _, tagIDStr := range tagIDs {
    tagID, err := uuid.Parse(tagIDStr)
    if err != nil {
        continue
    }
    _, err = h.DB.Exec(ctx,
        `INSERT INTO event_tags (event_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
        created.ID, tagID,
    )
    if err != nil {
        log.Printf("failed to insert tag %s for event %s: %v", tagIDStr, created.ID, err)
    }
}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// GetEvents godoc
// @Summary      Get a list of events
// @Description  Returns a list of events based on the provided query parameters. Supports filtering by host user ID, page ID, date range, city, tags, and search term. Pagination is supported via limit and offset.
// @Tags         events
// @Produce      json
// @Param        host_user_id  query     string  false  "Filter by host user ID"
// @Param        page_id       query     string  false  "Filter by page ID"
// @Param        from          query     string  false  "Filter events starting from this date (RFC3339 format)"
// @Param        to            query     string  false  "Filter events ending before this date (RFC3339 format)"
// @Param        city          query     string  false  "Filter by city"
// @Param        tag_id        query     []string false "Filter by tag IDs (can be specified multiple times)"
// @Param        search		   query     string  false  "Search term for event title or description"
// @Param        limit         query     int     false  "Number of events to return (default: 10)"
// @Param        offset        query     int     false  "Number of events to skip for pagination (default: 0)"
// @Success      200           {object}  map[string][]db.Event  "Returns a JSON object containing a list of events and the count"
// @Failure      400           {string}  string  "Bad Request: invalid query parameters"
// @Failure      500           {string}  string  "Internal Server Error: failed to fetch events"
// @Router       /events [get]
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

	var city *string
	if v := q.Get("city"); v != "" {
		city = &v
	}

	tagIDStrs := q["tag_id"]
	var tagIDs []uuid.UUID
	for _, s := range tagIDStrs {
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, "invalid tag_id: "+s, http.StatusBadRequest)
			return
		}
		tagIDs = append(tagIDs, id)
	}

	var search *string
	if v := q.Get("search"); v != "" {
		search = &v
	}

	limit := 10
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
		City: city,
		TagIDs: tagIDs,
		Search: search,
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

// GetEventByID godoc
// @Summary      Get event details by ID
// @Description  Returns the details of a specific event by its ID. The authenticated user must be either the host or an accepted attendee of the event.
// @Tags         events
// @Produce      json
// @Param        id   path      string  true  "Event ID"
// @Success      200  {object}  db.Event  "Returns the event details"
// @Failure      400  {string}  string  "Bad Request: invalid event ID"
// @Failure      401  {string}  string  "Unauthorized: user not authenticated"
// @Failure      403  {string}  string  "Forbidden: user is not authorized to view this event"
// @Failure      404  {string}  string  "Not Found: event does not exist"
// @Failure      500  {string}  string  "Internal Server Error: failed to fetch event"
// @Router       /events/{id} [get]
func (h *EventsHandler) GetEventByID(w http.ResponseWriter, r *http.Request) {
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
 
	event, err := db.GetEventByID(ctx, h.DB, eventID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch event", http.StatusInternalServerError)
		return
	}
 
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// JoinEvent godoc
// @Summary      Join an event
// @Description  Adds the authenticated user to the list of attendees for a specific event.
// @Tags         events
// @Produce      json
// @Param        id   path      string  true  "Event ID"
// @Success      204  {string}  string  "Successfully joined the event"
// @Failure      400  {string}  string  "Bad Request: invalid event ID"
// @Failure      401  {string}  string  "Unauthorized: user not authenticated"
// @Failure      403  {string}  string  "Forbidden: user is not authorized to join this event"
// @Failure      404  {string}  string  "Not Found: event does not exist"
// @Failure      500  {string}  string  "Internal Server Error: failed to join event"
// @Router       /events/{id}/join [post]
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

// CancelEvent godoc
// @Summary      Cancel an event
// @Description  Cancels a specific event if the authenticated user is the host.
// @Tags         events
// @Produce      json
// @Param        id   path      string  true  "Event ID"
// @Success      204  {string}  string  "Successfully cancelled the event"
// @Failure      400  {string}  string  "Bad Request: invalid event ID"
// @Failure      401  {string}  string  "Unauthorized: user not authenticated"
// @Failure      403  {string}  string  "Forbidden: user is not authorized to cancel this event"
// @Failure      404  {string}  string  "Not Found: event does not exist"
// @Failure      500  {string}  string  "Internal Server Error: failed to cancel event"
// @Router       /events/{id}/cancel [post]
func (h *EventsHandler) CancelEvent(w http.ResponseWriter, r *http.Request) {
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

	err = db.CancelEvent(r.Context(), h.DB, eventID, callerID)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "event not found", http.StatusNotFound)
		case errors.Is(err, db.ErrForbidden):
			http.Error(w, "only the host can cancel this event", http.StatusForbidden)
		case errors.Is(err, db.ErrAlreadyCancelled):
			http.Error(w, "event is already cancelled", http.StatusConflict)
		default:
			http.Error(w, "failed to cancel event", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetRegisteredEvents godoc
// @Summary      Get a list of events the user is registered for
// @Description  Returns a list of events that the currently authenticated user is registered for, either as a host or an accepted attendee.
// @Tags         events
// @Produce      json
// @Param        limit   query     int     false  "Number of events to return (default: 20)"
// @Param        offset  query     int     false  "Number of events to skip for pagination (default: 0)"
// @Success      200     {object}  map[string][]db.Event  "Returns a JSON object containing a list of registered events and the count"
// @Failure      401     {string}  string  "Unauthorized: user not authenticated"
// @Failure      500     {string}  string  "Internal Server Error: failed to fetch registered events"
// @Router       /events/registered [get]
func (h *EventsHandler) GetRegisteredEvents(w http.ResponseWriter, r *http.Request) {
	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()

	limit := 20
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	events, err := db.GetRegisteredEvents(r.Context(), h.DB, callerID, limit, offset)
	if err != nil {
		http.Error(w, "failed to fetch registered events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"events": events,
		"count":  len(events),
	})
}

// GetEventDashboard godoc
// @Summary      Get event dashboard
// @Description  Returns the dashboard data for a specific event. Only the host of the event can access this endpoint.
// @Tags         events
// @Produce      json
// @Param        id   path      string  true  "Event ID"
// @Success      200  {object}  db.EventDashboard  "Returns the event dashboard data"
// @Failure      400  {string}  string  "Bad Request: invalid event ID"
// @Failure      401  {string}  string  "Unauthorized: user not authenticated"
// @Failure      403  {string}  string  "Forbidden: only the host can view the dashboard"
// @Failure      404  {string}  string  "Not Found: event does not exist"
// @Failure      500  {string}  string  "Internal Server Error: failed to load dashboard"
// @Router       /events/{id}/dashboard [get]
func (h *EventsHandler) GetEventDashboard(w http.ResponseWriter, r *http.Request) {
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
	dashboard, err := db.GetEventDashboard(ctx, h.DB, eventID, callerID)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "event not found", http.StatusNotFound)
		case errors.Is(err, db.ErrForbidden):
			http.Error(w, "only the host can view the dashboard", http.StatusForbidden)
		default:
			http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboard)
}

// UpdateRegistrationStatus godoc
// @Summary      Update registration status for an event attendee
// @Description  Updates the registration status (pending, accepted, rejected) for a specific user attending an event. Only the host of the event can perform this action.
// @Tags         events
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Event ID"
// @Param        userID   path      string  true  "User ID of the attendee"
// @Param        body     body      map[string]string  true  "JSON body containing the new status" 
// @Success      204      {string}  string  "Successfully updated registration status"
// @Failure      400      {string}  string  "Bad Request: invalid input data"
// @Failure      401      {string}  string  "Unauthorized: user not authenticated"
// @Failure      403      {string}  string  "Forbidden: only the host can manage registrations"
// @Failure      404      {string}  string  "Not Found: event or registration does not exist"
// @Failure      500      {string}  string  "Internal Server Error: failed to update registration status"
// @Router       /events/{id}/registrations/{userID} [put]
func (h *EventsHandler) UpdateRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
 
	targetUserID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
 
	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
 
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
 
	validStatuses := map[string]bool{"pending": true, "accepted": true, "rejected": true}
	if !validStatuses[body.Status] {
		http.Error(w, "status must be one of: pending, accepted, rejected", http.StatusBadRequest)
		return
	}

	if body.Status == "rejected" {
		go func() {
			if err := h.FirebaseAuth.RevokeRefreshTokens(
				context.Background(),
				targetUserID.String(),
			); err != nil {
				log.Printf("failed to revoke firebase tokens for %s: %v", targetUserID, err)
			}
		}()
	}
 
	var hostUserID uuid.UUID
	err = h.DB.QueryRow(r.Context(),
		`SELECT host_user_id FROM events WHERE id = $1 AND deleted_at IS NULL`,
		eventID,
	).Scan(&hostUserID)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if hostUserID != callerID {
		http.Error(w, "only the host can manage registrations", http.StatusForbidden)
		return
	}
 
	err = db.UpdateRegistrationStatus(r.Context(), h.DB, eventID, targetUserID, body.Status)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "registration not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update registration", http.StatusInternalServerError)
		return
	}
 
	w.WriteHeader(http.StatusNoContent)
}

// LeaveEvent godoc
// @Summary      Leave an event
// @Description  Allows the authenticated user to leave a specific event they are registered for.
// @Tags         events
// @Produce      json
// @Param        id   path      string  true  "Event ID"
// @Success      204  {string}  string  "Successfully left the event"
// @Failure      400  {string}  string  "Bad Request: invalid event ID"
// @Failure      401  {string}  string  "Unauthorized: user not authenticated"
// @Failure      404  {string}  string  "Not Found: registration does not exist"
// @Failure      500  {string}  string  "Internal Server Error: failed to leave event"
// @Router       /events/{id}/leave [post]
func (h *EventsHandler) LeaveEvent(w http.ResponseWriter, r *http.Request) {
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

	err = db.LeaveEvent(r.Context(), h.DB, eventID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "registration not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to leave event", http.StatusInternalServerError)
		return
	}

	go func() {
		ctx := context.Background()

		hostID, eventTitle, err := db.GetEventHostAndTitle(ctx, h.DB, eventID)
		if err != nil {
			log.Printf("failed to get host and title for event %s: %v", eventID, err)
			return
		}

		hostToken, err := db.GetFCMToken(ctx, h.DB, hostID)
		if err != nil || hostToken == "" {
			log.Printf("failed to get FCM token for host %s: %v", hostID, err)
			return
		}

		var attendeeName string
		err = h.DB.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, callerID).Scan(&attendeeName)
		if err != nil {
			log.Printf("failed to get name for user %s: %v", callerID, err)
			return
		}

		notify.Send(ctx, notify.AttendeeLeft(hostToken, attendeeName, eventTitle, eventID.String()))
	}();

	w.WriteHeader(http.StatusNoContent)
}