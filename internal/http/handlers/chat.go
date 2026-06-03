package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
)

type ChatHandler struct {
	DB           *pgxpool.Pool
	FirebaseAuth *auth.Client
}

// POST /events/{id}/chat/token
func (h *ChatHandler) GetChatToken(w http.ResponseWriter, r *http.Request) {
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

	var count int
	err = h.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM events
			WHERE id = $1 AND host_user_id = $2 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM event_registrations
			WHERE event_id = $1 AND user_id = $2
			  AND status = 'accepted' AND deleted_at IS NULL
		) t
	`, eventID, callerID).Scan(&count)

	if err != nil || count == 0 {
		http.Error(w, "you must be an accepted attendee or host", http.StatusForbidden)
		return
	}

	token, err := h.FirebaseAuth.CustomToken(ctx, callerID.String())
	if err != nil {
		http.Error(w, "failed to create chat token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}

type ChatListEvent struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	ImageURL  *string    `json:"image_url"`
	EventStart time.Time `json:"event_start"`
	IsHost    bool       `json:"is_host"`
}

// GET /chats
func (h *ChatHandler) GetChatList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.DB.Query(ctx, `
			SELECT
				e.id,
				e.title,
				e.image_url,
				e.event_start,
				(e.host_user_id = $1) AS is_host
			FROM events e
			WHERE e.deleted_at IS NULL
			AND (
			e.host_user_id = $1
			OR
			EXISTS (
				SELECT 1 FROM event_registrations er
				WHERE er.event_id  = e.id
				AND er.user_id   = $1
				AND er.status    = 'accepted'
				AND er.deleted_at IS NULL
			)
			)
			ORDER BY e.event_start DESC
		`, callerID)
	if err != nil {
		http.Error(w, "failed to fetch chats", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	events := make([]ChatListEvent, 0)
	for rows.Next() {
		var ev ChatListEvent
		if err := rows.Scan(&ev.ID, &ev.Title, &ev.ImageURL, &ev.EventStart, &ev.IsHost); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		events = append(events, ev)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"chats": events})
}
