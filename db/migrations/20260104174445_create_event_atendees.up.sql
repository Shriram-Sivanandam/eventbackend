CREATE TABLE event_attendees (
  event_id UUID REFERENCES events(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,

  status TEXT CHECK (status IN ('pending', 'approved', 'rejected')) DEFAULT 'approved',

  joined_at TIMESTAMPTZ DEFAULT NOW(),

  PRIMARY KEY (event_id, user_id)
);