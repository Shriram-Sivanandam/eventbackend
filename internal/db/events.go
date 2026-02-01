package db

import (
	"context"
	"fmt"
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

type GetEventParams struct {
	HostUserID *uuid.UUID
	PageID *uuid.UUID
	From *time.Time
	To *time.Time
	Limit int
	Offset int
}

func GetEvents (ctx context.Context, pool *pgxpool.Pool, p GetEventParams) ([]Event, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	} else if p.Limit > 100 {
		p.Limit = 100
	}

	query := `SELECT id, host_user_id, host_page_id, title, description, location, event_start, event_end, price, capacity, created_at FROM events WHERE 1=1`

	args := []any{}
	argN := 1

	if p.HostUserID != nil {
		query += " AND host_user_id = $" + itoa(argN)
		args = append(args, *p.HostUserID)
		argN++
	}
	if p.PageID != nil {
		query += " AND host_page_id = $" + itoa(argN)
		args = append(args, *p.PageID)
		argN++
	}
	if p.From != nil {
		query += " AND event_start >= $" + itoa(argN)
		args = append(args, *p.From)
		argN++
	}
	if p.To != nil {
		query += " AND event_start <= $" + itoa(argN)
		args = append(args, *p.To)
		argN++
	}

	query += " ORDER BY event_start ASC"
	query += " LIMIT $" + itoa(argN)
	args = append(args, p.Limit)
	argN++

	query += " OFFSET $" + itoa(argN)
	args = append(args, p.Offset)
	argN++

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0, p.Limit)

	for rows.Next() {
		var e Event

		if err := rows.Scan(&e.ID, &e.HostUserID, &e.HostPageID, &e.Title, &e.Description, &e.Location, &e.EventStart, &e.EventEnd, &e.Price, &e.Capacity, &e.CreatedAt); err != nil {
			return nil, err
		}

		events = append(events, e)

	}

	return events, nil

}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
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