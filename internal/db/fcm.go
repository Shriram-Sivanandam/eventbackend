package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SaveFCMToken(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, token string) error {
	_, err := pool.Exec(ctx, `
		UPDATE users
		SET fcm_token      = $1,
		    fcm_updated_at = NOW()
		WHERE id = $2
	`, token, userID)
	return err
}

func GetFCMToken(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) (string, error) {
	var token *string
	err := pool.QueryRow(ctx,
		`SELECT fcm_token FROM users WHERE id = $1`,
		userID,
	).Scan(&token)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", nil
	}
	return *token, nil
}


func GetFCMTokens(ctx context.Context, pool *pgxpool.Pool, userIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, fcm_token FROM users
		 WHERE id = ANY($1) AND fcm_token IS NOT NULL`,
		userIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]string)
	for rows.Next() {
		var id    uuid.UUID
		var token string
		if err := rows.Scan(&id, &token); err != nil {
			return nil, err
		}
		result[id] = token
	}
	return result, rows.Err()
}