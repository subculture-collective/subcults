-- Rollback stream_participants table and participant_count column

-- Drop indexes first
DROP INDEX IF EXISTS idx_stream_active_participant_count;
DROP INDEX IF EXISTS idx_stream_participants_joined;
DROP INDEX IF EXISTS idx_stream_participants_user;
DROP INDEX IF EXISTS idx_stream_participants_session;

-- Remove denormalized column
ALTER TABLE stream_sessions
DROP COLUMN IF EXISTS active_participant_count;

-- Drop table
DROP TABLE IF EXISTS stream_participants;
