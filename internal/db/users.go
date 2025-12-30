package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Email string `json:"email"`
}

func GetUsers (ctx context.Context, pool *pgxpool.Pool) ([]User, error) {
	rows, err := pool.Query (ctx, `SELECT id, name, email FROM users ORDER BY created_at DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var u User

		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}

		users = append(users, u)
	}

	return users, nil
}