CREATE TABLE IF NOT EXISTS event_ratings (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id    UUID NOT NULL REFERENCES events(id),
  rater_id    UUID NOT NULL REFERENCES users(id), 
  ratee_id    UUID NOT NULL REFERENCES users(id), 
  rating_type TEXT NOT NULL CHECK (rating_type IN ('host', 'attendee')),
  score       INT  NOT NULL CHECK (score BETWEEN 1 AND 5),
  comment     TEXT,
  created_at  TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE (event_id, rater_id, rating_type)
);