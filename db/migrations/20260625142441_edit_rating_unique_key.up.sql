ALTER TABLE event_ratings DROP CONSTRAINT event_ratings_event_id_rater_id_rating_type_key;

ALTER TABLE event_ratings
  ADD CONSTRAINT event_ratings_unique_rating
  UNIQUE (event_id, rater_id, ratee_id, rating_type);