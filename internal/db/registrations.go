package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func JoinEvent(ctx context.Context, pool *pgxpool.Pool, eventID, userID uuid.UUID) error {
	_, err := pool.Exec(ctx, 
		`INSERT INTO event_registrations (event_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (event_id, user_id) DO NOTHING`,
		eventID, userID,
	)

	return err
}

func UpdateRegistrationStatus(ctx context.Context, pool *pgxpool.Pool, eventID, userID uuid.UUID, status string) error {
	tag, err := pool.Exec(ctx, `
		UPDATE event_registrations
		SET status = $1
		WHERE event_id = $2
		  AND user_id = $3
		  AND deleted_at IS NULL
	`, status, eventID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func LeaveEvent(ctx context.Context, pool *pgxpool.Pool, eventID, callerID uuid.UUID) error {
	tag, err := pool.Exec(ctx,
		`DELETE FROM event_registrations
		WHERE event_id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL`,
		eventID, callerID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func GetEventHostAndTitle(ctx context.Context, pool *pgxpool.Pool, eventID uuid.UUID) (uuid.UUID, string, error) {
	var hostID uuid.UUID
	var title string
	err := pool.QueryRow(ctx,
		`SELECT host_user_id, title FROM events WHERE id = $1 AND deleted_at IS NULL`,
		eventID,
	).Scan(&hostID, &title)
	if err != nil {	
		return uuid.Nil, "", err
	}
	return hostID, title, nil
}