package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	City *string `json:"city"`
	AddressLineOne *string `json:"address_line_one"`
	Pincode *string `json:"pincode"`
	MapsLink *string `json:"maps_link"`
	DurationMinutes *int `json:"duration_minutes"`
	ThingsToBring *string `json:"things_to_bring"`
	ThingsProvided *string `json:"things_provided"`
	ImageURL *string `json:"image_url"`
	Joined bool `json:"joined"`
}

type GetEventParams struct {
	HostUserID *uuid.UUID
	PageID *uuid.UUID
	From *time.Time
	To *time.Time
	Limit int
	Offset int
}

var ErrNotFound = errors.New("event not found")
var ErrForbidden = errors.New("you are not the host of this event")
var ErrAlreadyCancelled = errors.New("event is already cancelled")

func GetEvents (ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, p GetEventParams) ([]Event, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	} else if p.Limit > 100 {
		p.Limit = 100
	}

	query := `SELECT e.id, e.host_user_id, e.host_page_id, e.title, e.description, e.location, e.event_start, e.event_end, e.price, e.capacity, e.created_at, e.city, e.address_line_one, e.pincode, e.maps_link, e.duration_minutes, e.things_to_bring, e.things_provided, e.image_url,
		EXISTS (
			SELECT 1 FROM event_registrations er
			WHERE er.event_id = e.id
			AND er.user_id = $1
			AND er.deleted_at IS NULL
		) AS JOINED	
		FROM events e WHERE e.deleted_at IS NULL`

	args := []any{}
	args = append(args, userID)
	argN := 2

	if p.HostUserID != nil {
		query += " AND host_user_id = $" + itoa(argN)
		args = append(args, *p.HostUserID)
		argN++
	}
	if p.HostUserID == nil || (p.HostUserID != nil && *p.HostUserID != userID) {
		query += " AND host_user_id <> $" + itoa(argN)
		args = append(args, userID)
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

		if err := rows.Scan(&e.ID, &e.HostUserID, &e.HostPageID, &e.Title, &e.Description, &e.Location, &e.EventStart, &e.EventEnd, &e.Price, &e.Capacity, &e.CreatedAt, &e.City, &e.AddressLineOne, &e.Pincode, &e.MapsLink, &e.DurationMinutes, &e.ThingsToBring, &e.ThingsProvided, &e.ImageURL, &e.Joined); err != nil {
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
									capacity,
									city,
									address_line_one,
									pincode,
									maps_link,
									duration_minutes,
									things_to_bring,
									things_provided,
									image_url
									)
									VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
									RETURNING
									id,
									created_at
									`
	err := pool.QueryRow(ctx, query, e.HostUserID, e.HostPageID, e.Title, e.Description, e.Location, e.EventStart, e.EventEnd, e.Price, e.Capacity, e.City, e.AddressLineOne, e.Pincode, e.MapsLink, e.DurationMinutes, e.ThingsToBring, e.ThingsProvided, e.ImageURL).Scan(&e.ID, &e.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &e, nil
}

func CancelEvent(ctx context.Context, pool *pgxpool.Pool, eventID, callerID uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var hostUserID uuid.UUID
	var deletedAt *time.Time

	err = tx.QueryRow(ctx,
		`SELECT host_user_id, deleted_at FROM events WHERE id = $1`,
		eventID,
	).Scan(&hostUserID, &deletedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if hostUserID != callerID {
		return ErrForbidden
	}
	if deletedAt != nil {
		return ErrAlreadyCancelled
	}

	now := time.Now()

	_, err = tx.Exec(ctx,
		`UPDATE events SET deleted_at = $1 WHERE id = $2`,
		now, eventID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE event_registrations SET deleted_at = $1 WHERE event_id = $2 AND deleted_at IS NULL`,
		now, eventID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func GetRegisteredEvents(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, limit, offset int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	query := `
		SELECT
			e.id, e.host_user_id, e.host_page_id,
			e.title, e.description, e.location,
			e.event_start, e.event_end, e.price, e.capacity,
			e.created_at, e.city, e.address_line_one, e.pincode,
			e.maps_link, e.duration_minutes, e.things_to_bring,
			e.things_provided, e.image_url,
			true AS joined
		FROM events e
		INNER JOIN event_registrations er
			ON er.event_id = e.id
			AND er.user_id = $1
			AND er.deleted_at IS NULL
		WHERE e.deleted_at IS NULL
		ORDER BY e.event_start ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID, &e.HostUserID, &e.HostPageID,
			&e.Title, &e.Description, &e.Location,
			&e.EventStart, &e.EventEnd, &e.Price, &e.Capacity,
			&e.CreatedAt, &e.City, &e.AddressLineOne, &e.Pincode,
			&e.MapsLink, &e.DurationMinutes, &e.ThingsToBring,
			&e.ThingsProvided, &e.ImageURL, &e.Joined,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}