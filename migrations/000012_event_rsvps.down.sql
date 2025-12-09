-- Migration rollback: Remove event_rsvps table

-- Drop indexes first
DROP INDEX IF EXISTS idx_event_rsvps_user_id;
DROP INDEX IF EXISTS idx_event_rsvps_event_id;

-- Drop the table
DROP TABLE IF EXISTS event_rsvps;
