package db

import (
	"context"
	"encoding/json"
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
	RegistrantCount int `json:"registrant_count"`
	Tags []EventTag `json:"tags"`
	HasRated bool `json:"has_rated"`
}

type EventDashboard struct {
	Event           Event        `json:"event"`
	TotalRegistered int          `json:"total_registered"`
	Accepted        int          `json:"accepted"`
	Pending         int          `json:"pending"`
	Rejected        int          `json:"rejected"`
	Registrants     []Registrant `json:"registrants"`
}

type Registrant struct {
	RegistrationID string    `json:"registration_id"`
	UserID         uuid.UUID `json:"user_id"`
	Name           *string   `json:"name"`
	Email          string    `json:"email"`
	AvatarURL      *string   `json:"avatar_url"`
	Status         string    `json:"status"`
	RegisteredAt   string    `json:"registered_at"`
	HasRated       bool      `json:"has_rated"`
}

type DashboardStats struct {
	TotalRegistrants    int `json:"total_registrants"`
	PendingCount        int `json:"pending_count"`
	AcceptedCount       int `json:"accepted_count"`
	RejectedCount       int `json:"rejected_count"`
}

type GetEventParams struct {
	HostUserID *uuid.UUID
	PageID *uuid.UUID
	From *time.Time
	To *time.Time
	City *string
	TagIDs []uuid.UUID
	Search *string
	Limit int
	Offset int
}

var ErrNotFound = errors.New("event not found")
var ErrForbidden = errors.New("you are not the host of this event")
var ErrAlreadyCancelled = errors.New("event is already cancelled")

type EventTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func unmarshalTags(data []byte, dest *[]EventTag) error {
	if len(data) == 0 || string(data) == "null" {
		*dest = []EventTag{}
		return nil
	}
	return json.Unmarshal(data, dest)
}

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
		) AS JOINED,	
		(
			SELECT COUNT(*) FROM event_registrations er
			WHERE er.event_id = e.id
			AND er.deleted_at IS NULL
		) AS registrant_count,
		 COALESCE(
				(
					SELECT json_agg(json_build_object('id', t.id::text, 'name', t.name))
					FROM event_tags et
					JOIN tags t ON t.id = et.tag_id
					WHERE et.event_id = e.id
				),
				'[]'::json
			) AS tags
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
	if p.City != nil {
		query += " AND city ILIKE $" + itoa(argN)
		args = append(args, "%"+*p.City+"%")
		argN++
	}
	if len(p.TagIDs) > 0 {
		query += ` AND EXISTS (
			SELECT 1 FROM event_tags et
			WHERE et.event_id = e.id
			AND et.tag_id = ANY($` + itoa(argN) + `::uuid[])
		)`
		args = append(args, p.TagIDs)
		argN++
	}
	if p.Search != nil {
    query += " AND (e.title ILIKE $" + itoa(argN) +
             " OR e.location ILIKE $" + itoa(argN) +
             " OR e.city ILIKE $" + itoa(argN) + ")"
    args = append(args, "%"+*p.Search+"%")
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
		var tagsJSON []byte

		if err := rows.Scan(&e.ID, &e.HostUserID, &e.HostPageID, &e.Title, &e.Description, &e.Location, &e.EventStart, &e.EventEnd, &e.Price, &e.Capacity, &e.CreatedAt, &e.City, &e.AddressLineOne, &e.Pincode, &e.MapsLink, &e.DurationMinutes, &e.ThingsToBring, &e.ThingsProvided, &e.ImageURL, &e.Joined, &e.RegistrantCount, &tagsJSON); err != nil {
			return nil, err
		}

		if err := unmarshalTags(tagsJSON, &e.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags for event %s: %w", e.ID, err)
		}

		events = append(events, e)

	}

	return events, nil

}

type EventDetail struct {
	Event
 
	HostName         *string  `json:"host_name"`
	HostAvatarURL    *string  `json:"host_avatar_url"`
	HostingRating    *float64 `json:"hosting_rating"`
	HostTotalHosted  int      `json:"host_total_hosted"`
}

func GetEventByID(ctx context.Context, pool *pgxpool.Pool, eventID, callerID uuid.UUID) (*EventDetail, error) {
	var d EventDetail
 
	err := pool.QueryRow(ctx, `
		SELECT
			e.id,
			e.host_user_id,
			e.host_page_id,
			e.title,
			e.description,
			e.location,
			e.event_start,
			e.event_end,
			e.price,
			e.capacity,
			e.created_at,
			e.city,
			e.address_line_one,
			e.pincode,
			e.maps_link,
			e.duration_minutes,
			e.things_to_bring,
			e.things_provided,
			e.image_url,
 
			EXISTS (
				SELECT 1 FROM event_registrations er
				WHERE er.event_id = e.id
				  AND er.user_id  = $2
				  AND er.deleted_at IS NULL
			) AS joined,
 
			(
				SELECT COUNT(*) FROM event_registrations er
				WHERE er.event_id = e.id
				  AND er.deleted_at IS NULL
				  AND er.status != 'rejected'
			) AS registrant_count,
 
			u.name        AS host_name,
			u.avatar_url  AS host_avatar_url,
 
			(
				SELECT ROUND(AVG(score)::numeric, 1)
				FROM event_ratings
				WHERE ratee_id   = u.id
				  AND rating_type = 'host'
			) AS hosting_rating,
 
			(
				SELECT COUNT(*)
				FROM events
				WHERE host_user_id = u.id
				  AND deleted_at IS NULL
			) AS host_total_hosted
 
		FROM events e
		LEFT JOIN users u ON u.id = e.host_user_id
		WHERE e.id = $1
		  AND e.deleted_at IS NULL
	`, eventID, callerID).Scan(
		&d.ID,
		&d.HostUserID,
		&d.HostPageID,
		&d.Title,
		&d.Description,
		&d.Location,
		&d.EventStart,
		&d.EventEnd,
		&d.Price,
		&d.Capacity,
		&d.CreatedAt,
		&d.City,
		&d.AddressLineOne,
		&d.Pincode,
		&d.MapsLink,
		&d.DurationMinutes,
		&d.ThingsToBring,
		&d.ThingsProvided,
		&d.ImageURL,
		&d.Joined,
		&d.RegistrantCount,
		&d.HostName,
		&d.HostAvatarURL,
		&d.HostingRating,
		&d.HostTotalHosted,
	)
 
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
 
	return &d, nil
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
			true AS joined,
			EXISTS (
				SELECT 1 FROM event_ratings r
				WHERE r.event_id = e.id
				AND r.rater_id = $1
				AND r.rating_type = 'host'
			) AS has_rated
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
			&e.ThingsProvided, &e.ImageURL, &e.Joined, &e.HasRated,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func GetEventDashboard(ctx context.Context, pool *pgxpool.Pool, eventID, callerID uuid.UUID) (*EventDashboard, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
 
	var e Event
	err = tx.QueryRow(ctx, `
		SELECT
			e.id, e.host_user_id, e.host_page_id,
			e.title, e.description, e.location,
			e.event_start, e.event_end, e.price, e.capacity,
			e.created_at, e.city, e.address_line_one, e.pincode,
			e.maps_link, e.duration_minutes, e.things_to_bring,
			e.things_provided, e.image_url,
			true AS joined,
			(
				SELECT COUNT(*) FROM event_registrations er
				WHERE er.event_id = e.id
				  AND er.deleted_at IS NULL
				  AND er.status != 'rejected'
			) AS registrant_count
		FROM events e
		WHERE e.id = $1
		  AND e.deleted_at IS NULL
	`, eventID).Scan(
		&e.ID, &e.HostUserID, &e.HostPageID,
		&e.Title, &e.Description, &e.Location,
		&e.EventStart, &e.EventEnd, &e.Price, &e.Capacity,
		&e.CreatedAt, &e.City, &e.AddressLineOne, &e.Pincode,
		&e.MapsLink, &e.DurationMinutes, &e.ThingsToBring,
		&e.ThingsProvided, &e.ImageURL,
		&e.Joined, &e.RegistrantCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if e.HostUserID == nil || *e.HostUserID != callerID {
		return nil, ErrForbidden
	}

	rows, err := tx.Query(ctx, `
		SELECT
			er.id::text,
			u.id,
			u.name,
			u.email,
			u.avatar_url,
			er.status,
			to_char(er.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS registered_at,
			EXISTS (
				SELECT 1 FROM event_ratings r
				WHERE r.event_id    = er.event_id
				  AND r.rater_id    = $2
				  AND r.ratee_id    = er.user_id
				  AND r.rating_type = 'attendee'
			) AS has_rated
		FROM event_registrations er
		INNER JOIN users u ON u.id = er.user_id
		WHERE er.event_id = $1
		  AND er.deleted_at IS NULL
		ORDER BY er.created_at ASC
	`, eventID, &e.HostUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
 
	registrants := make([]Registrant, 0)
	dashboard := &EventDashboard{Event: e}
 
	for rows.Next() {
		var r Registrant
		if err := rows.Scan(
			&r.RegistrationID,
			&r.UserID,
			&r.Name,
			&r.Email,
			&r.AvatarURL,
			&r.Status,
			&r.RegisteredAt,
			&r.HasRated,
		); err != nil {
			return nil, err
		}
		registrants = append(registrants, r)
 
		switch r.Status {
		case "pending":
			dashboard.Pending++
		case "accepted":
			dashboard.Accepted++
		case "rejected":
			dashboard.Rejected++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
 
	dashboard.Registrants = registrants
	dashboard.TotalRegistered = len(registrants)
 
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
 
	return dashboard, nil
}