ALTER TABLE event_registrations DROP CONSTRAINT IF EXISTS event_registrations_event_id_user_id_key;

CREATE UNIQUE INDEX event_registrations_unique_active
ON event_registrations (event_id, user_id)
WHERE deleted_at IS NULL;