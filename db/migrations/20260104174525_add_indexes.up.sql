CREATE INDEX idx_events_start ON events(event_start);
CREATE INDEX idx_events_host_user ON events(host_user_id);
CREATE INDEX idx_events_host_page ON events(host_page_id);
CREATE INDEX idx_event_attendees_user ON event_attendees(user_id);
