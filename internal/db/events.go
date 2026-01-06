package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID string `json:"id"`
	HOSTID string `json:"host_user_id"`
	HOSTPAGEID string `json:"host_page_id"`
	TITLE string `json:"title"`
	DESCRIPTION string `json:"description"`
	LOCATION string `json:"location"`
	EVENTSTART string `json:"event_start"`
	EVENTEND string `json:"event_end"`
	PRICE string `json:"price"`
	CAPACITY string `json:"capacity"`
}

func getEvents (ctx context.Context, pool *pgxpool.Pool) ([]Event, error) {
	rows, err := pool.Query(ctx, `SELECT id, host_user_id, host_page_id, title, description, location, event_start, event_end, price, capacity FROM events ORDER BY created_at DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event

	for rows.Next() {
		var u Event

		if err := rows.Scan(&u.ID, &u.HOSTID, &u.HOSTPAGEID, &u.TITLE, &u.DESCRIPTION, &u.LOCATION, &u.EVENTSTART, &u.EVENTEND, &u.PRICE, &u.CAPACITY); err != nil {
			return nil, err
		}

		events = append(events, u)

	}

	return events, nil

}