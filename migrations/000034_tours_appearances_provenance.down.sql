DROP TABLE IF EXISTS entity_assertions;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS appearances;
DROP TABLE IF EXISTS tour_acts;
DROP TABLE IF EXISTS tours;
DROP TABLE IF EXISTS event_hosts;

DROP INDEX IF EXISTS idx_events_kind_time;
DROP INDEX IF EXISTS idx_events_venue_time;
DROP INDEX IF EXISTS idx_events_place_time;

ALTER TABLE events DROP COLUMN IF EXISTS kind;
ALTER TABLE events DROP COLUMN IF EXISTS venue_id;
ALTER TABLE events DROP COLUMN IF EXISTS place_id;
