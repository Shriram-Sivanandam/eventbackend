package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/notify"
)

func StartReminderJob(pool *pgxpool.Pool) {
	go func() {
		for {
			if err := sendReminders(pool); err != nil {
				log.Printf("reminder job error: %v", err)
			}
			time.Sleep(10 * time.Minute)
		}
	}()
}

func sendReminders(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()
	windowStart := now.Add(55 * time.Minute)
	windowEnd   := now.Add(65 * time.Minute)

	rows, err := pool.Query(ctx, `
		SELECT
			e.id,
			e.title,
			er.user_id
		FROM events e
		INNER JOIN event_registrations er
			ON er.event_id  = e.id
			AND er.status   = 'accepted'
			AND er.deleted_at IS NULL
		WHERE e.event_start >= $1
		  AND e.event_start <= $2
		  AND e.deleted_at  IS NULL
		  AND er.reminder_sent_at IS NULL
	`, windowStart, windowEnd)
	if err != nil {
		return err
	}
	defer rows.Close()

	type reminderTarget struct {
		eventID    uuid.UUID
		eventTitle string
		userID     uuid.UUID
	}

	var targets []reminderTarget
	for rows.Next() {
		var t reminderTarget
		if err := rows.Scan(&t.eventID, &t.eventTitle, &t.userID); err != nil {
			return err
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(targets) == 0 {
		return nil
	}

	userIDSet := make(map[uuid.UUID]bool)
	for _, t := range targets {
		userIDSet[t.userID] = true
	}
	userIDs := make([]uuid.UUID, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}

	tokens, err := db.GetFCMTokens(ctx, pool, userIDs)
	if err != nil {
		return err
	}

	for _, t := range targets {
		token, ok := tokens[t.userID]
		if !ok {
			continue
		}

		notify.Send(ctx, notify.EventReminder(token, t.eventTitle, t.eventID.String()))

		pool.Exec(ctx, `
			UPDATE event_registrations
			SET reminder_sent_at = NOW()
			WHERE event_id = $1 AND user_id = $2
		`, t.eventID, t.userID)
	}

	log.Printf("reminder job: sent %d reminders", len(targets))
	return nil
}