package db

import (
	"context"

	"github.com/google/uuid"
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

type UpdateProfileParams struct {
	Name      *string
	Phone     *string
	Bio       *string
	City      *string
	AvatarURL *string
	Gender    *string 
	Age       *int
}
 
func UpdateProfile(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, p UpdateProfileParams) error {
	_, err := pool.Exec(ctx, `
		UPDATE users SET
			name       = COALESCE($1,  name),
			phone      = COALESCE($2,  phone),
			bio        = COALESCE($3,  bio),
			city       = COALESCE($4,  city),
			avatar_url = COALESCE($5,  avatar_url),
			gender     = COALESCE($6,  gender),
			age        = COALESCE($7,  age),
			updated_at = NOW()
		WHERE id = $8
	`,
		p.Name, p.Phone, p.Bio, p.City,
		p.AvatarURL, p.Gender, p.Age, userID,
	)
	return err
}