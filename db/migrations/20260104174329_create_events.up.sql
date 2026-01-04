CREATE TABLE events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  host_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  host_page_id UUID REFERENCES pages(id) ON DELETE SET NULL,

  title TEXT NOT NULL,
  description TEXT,

  location TEXT,
  event_start TIMESTAMPTZ NOT NULL,
  event_end TIMESTAMPTZ,

  price INTEGER DEFAULT 0,

  capacity INTEGER,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CHECK (
    host_user_id IS NOT NULL OR host_page_id IS NOT NULL
  )
);
