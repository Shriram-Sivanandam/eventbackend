ALTER TABLE event_registrations 
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending';