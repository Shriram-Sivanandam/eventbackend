package db

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UnratedEvent struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	EventStart time.Time `json:"event_start"`
	ImageURL   *string   `json:"image_url"`
	HostUserID uuid.UUID `json:"host_user_id"`
	HostName   *string   `json:"host_name"`
	HostAvatar *string   `json:"host_avatar"`
}

func GetUnratedEvents(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]UnratedEvent, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			e.id,
			e.title,
			e.event_start,
			e.image_url,
			e.host_user_id,
			u.name       AS host_name,
			u.avatar_url AS host_avatar
		FROM events e
		INNER JOIN event_registrations er
			ON  er.event_id  = e.id
			AND er.user_id   = $1
			AND er.deleted_at IS NULL
			AND er.status    = 'accepted'
			AND er.rating_prompt_seen_at IS NULL
		LEFT JOIN users u ON u.id = e.host_user_id
		WHERE e.event_start < NOW()
		  AND e.deleted_at  IS NULL
		  AND e.host_user_id <> $1
		  AND NOT EXISTS (
				SELECT 1 FROM event_ratings r
				WHERE r.event_id    = e.id
				  AND r.rater_id    = $1
				  AND r.rating_type = 'host'
		  )
		ORDER BY e.event_start DESC
		LIMIT 10
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
 
	events := make([]UnratedEvent, 0)
	for rows.Next() {
		var ev UnratedEvent
		if err := rows.Scan(
			&ev.ID, &ev.Title, &ev.EventStart, &ev.ImageURL,
			&ev.HostUserID, &ev.HostName, &ev.HostAvatar,
		); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

func SubmitRating(
	ctx context.Context,
	pool *pgxpool.Pool,
	eventID, raterID, rateeID uuid.UUID,
	ratingType string,
	score int,
	comment *string,
	tags []string,
) error {
	var tagsStr *string
	if len(tags) > 0 {
		joined := strings.Join(tags, ",")
		tagsStr = &joined
	}
 
	_, err := pool.Exec(ctx, `
		INSERT INTO event_ratings (event_id, rater_id, ratee_id, rating_type, score, comment, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (event_id, rater_id, rating_type)
		DO UPDATE SET
			score   = EXCLUDED.score,
			comment = EXCLUDED.comment,
			tags    = EXCLUDED.tags
	`, eventID, raterID, rateeID, ratingType, score, comment, tagsStr)
	return err
}

func DismissRatingPrompt(ctx context.Context, pool *pgxpool.Pool, eventID, userID uuid.UUID) error {
	_, err := pool.Exec(ctx, `
		UPDATE event_registrations
		SET rating_prompt_seen_at = NOW()
		WHERE event_id   = $1
		  AND user_id    = $2
		  AND deleted_at IS NULL
	`, eventID, userID)
	return err
}