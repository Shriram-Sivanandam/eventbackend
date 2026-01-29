package db

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID uuid.UUID `json:"id"`
	HostUserID *uuid.UUID `json:"host_user_id"`
	HostPageID *uuid.UUID `json:"host_page_id"`
	Title string `json:"title"`
	Description *string `json:"description"`
	Location *string `json:"location"`
	EventStart time.Time `json:"event_start"`
	EventEnd *time.Time `json:"event_end"`
	Price int `json:"price"`
	Capacity *int `json:"capacity"`
	CreatedAt  time.Time  `json:"created_at"`
}

func GetEvents (ctx context.Context, pool *pgxpool.Pool) ([]Event, error) {
	rows, err := pool.Query(ctx, `SELECT id, host_user_id, host_page_id, title, description, location, event_start, event_end, price, capacity FROM events ORDER BY created_at DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event

	for rows.Next() {
		var u Event

		if err := rows.Scan(&u.ID, &u.HostUserID, &u.HostPageID, &u.Title, &u.Description, &u.Location, &u.EventStart, &u.EventEnd, &u.Price, &u.Capacity); err != nil {
			return nil, err
		}

		events = append(events, u)

	}

	return events, nil

}

func CreateEvent (ctx context.Context, pool *pgxpool.Pool, e Event) (*Event, error) {
	query := `INSERT INTO events (
									host_user_id,
									host_page_id,
									title,
									description,
									location,
									event_start,
									event_end,
									price,
									capacity
									)
									VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
									RETURNING
									id,
									created_at
									`
	err := pool.QueryRow(ctx, query, e.HostUserID, e.HostPageID, e.Title, e.Description, e.Location, e.EventStart, e.EventEnd, e.Price, e.Capacity).Scan(&e.ID, &e.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &e, nil
}