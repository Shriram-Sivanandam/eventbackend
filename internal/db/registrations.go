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