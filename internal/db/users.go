package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	Name               *string
	Phone              *string
	Bio                *string
	City               *string
	Gender             *string
	Age                *int
	DateOfBirth        *string
	AvatarURL          *string
	OnboardingComplete *bool
}

func UpdateProfile(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, p UpdateProfileParams) error {
	_, err := pool.Exec(ctx, `
		UPDATE users SET
			name                = COALESCE($1,  name),
			phone               = COALESCE($2,  phone),
			bio                 = COALESCE($3,  bio),
			city                = COALESCE($4,  city),
			gender              = COALESCE($5,  gender),
			age                 = COALESCE($6,  age),
			date_of_birth       = COALESCE($7::date, date_of_birth),
			avatar_url          = COALESCE($8,  avatar_url),
			onboarding_complete = COALESCE($9,  onboarding_complete),
			updated_at          = NOW()
		WHERE id = $10
	`,
		p.Name,
		p.Phone,
		p.Bio,
		p.City,
		p.Gender,
		p.Age,
		p.DateOfBirth,
		p.AvatarURL,
		p.OnboardingComplete,
		userID,
	)
	return err
}

type HostProfile struct {
	UserID       uuid.UUID  `json:"user_id"`
	Name         *string    `json:"name"`
	Email        string     `json:"email"`
	AvatarURL    *string    `json:"avatar_url"`
	Bio          *string    `json:"bio"`
	City         *string    `json:"city"`
	Gender       *string    `json:"gender"`
	Age          *int       `json:"age"`
	HostingRating  *float64 `json:"hosting_rating"` 
	AttendeeRating *float64 `json:"attendee_rating"`
	TotalHosted    int      `json:"total_hosted"`
	TotalAttended  int      `json:"total_attended"`
	TotalRatings   int      `json:"total_ratings"`
	PastEvents []HostedEvent `json:"past_events"`
}

type HostedEvent struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Location    *string    `json:"location"`
	City        *string    `json:"city"`
	EventStart  time.Time  `json:"event_start"`
	ImageURL    *string    `json:"image_url"`
	Price       int        `json:"price"`
	AvgRating   *float64   `json:"avg_rating"`
	RatingCount int        `json:"rating_count"`
}

func GetHostProfile(ctx context.Context, pool *pgxpool.Pool, hostID uuid.UUID) (*HostProfile, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
 
	var p HostProfile
	err = tx.QueryRow(ctx, `
		SELECT
			u.id,
			u.name,
			u.email,
			u.avatar_url,
			u.bio,
			u.city,
			u.gender,
			u.age,
			(
				SELECT ROUND(AVG(score)::numeric, 1)
				FROM event_ratings
				WHERE ratee_id = u.id AND rating_type = 'host'
			),
			(
				SELECT ROUND(AVG(score)::numeric, 1)
				FROM event_ratings
				WHERE ratee_id = u.id AND rating_type = 'attendee'
			),
			(
				SELECT COUNT(*)
				FROM events
				WHERE host_user_id = u.id AND deleted_at IS NULL
			),
			(
				SELECT COUNT(*)
				FROM event_registrations er
				INNER JOIN events e ON e.id = er.event_id
				WHERE er.user_id = u.id
				  AND er.deleted_at IS NULL
				  AND er.status = 'accepted'
				  AND e.event_start < NOW()
				  AND e.deleted_at IS NULL
			),
			(
				SELECT COUNT(*)
				FROM event_ratings
				WHERE ratee_id = u.id AND rating_type = 'host'
			)
 
		FROM users u
		WHERE u.id = $1
	`, hostID).Scan(
		&p.UserID,
		&p.Name,
		&p.Email,
		&p.AvatarURL,
		&p.Bio,
		&p.City,
		&p.Gender,
		&p.Age,
		&p.HostingRating,
		&p.AttendeeRating,
		&p.TotalHosted,
		&p.TotalAttended,
		&p.TotalRatings,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
 
	rows, err := tx.Query(ctx, `
		SELECT
			e.id,
			e.title,
			e.location,
			e.city,
			e.event_start,
			e.image_url,
			e.price,
			ROUND(AVG(er.score)::numeric, 1) AS avg_rating,
			COUNT(er.id)                     AS rating_count
		FROM events e
		LEFT JOIN event_ratings er
			ON er.event_id = e.id
			AND er.rating_type = 'host'
		WHERE e.host_user_id = $1
		  AND e.deleted_at IS NULL
		  AND e.event_start < NOW()
		GROUP BY e.id
		ORDER BY e.event_start DESC
		LIMIT 20
	`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
 
	p.PastEvents = make([]HostedEvent, 0)
	for rows.Next() {
		var ev HostedEvent
		if err := rows.Scan(
			&ev.ID,
			&ev.Title,
			&ev.Location,
			&ev.City,
			&ev.EventStart,
			&ev.ImageURL,
			&ev.Price,
			&ev.AvgRating,
			&ev.RatingCount,
		); err != nil {
			return nil, err
		}
		p.PastEvents = append(p.PastEvents, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
 
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
 
	return &p, nil
}