-- Migration: Enhance events table for schedule-based discovery
-- Adds: title (rename from name), tags, status, stream_session_id, FTS column
-- Ensures: coarse_geohash NOT NULL, GIN index on tags

-- Step 1: Rename name to title (per issue specification)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'events' AND column_name = 'name'
    ) THEN
        ALTER TABLE events RENAME COLUMN name TO title;
    END IF;
END $$;

-- Step 2: Add tags column for categorization
ALTER TABLE events ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';

-- Step 3: Add status column for event lifecycle tracking
ALTER TABLE events ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'scheduled';

-- Step 4: Add stream_session_id foreign key for live streaming
ALTER TABLE events ADD COLUMN IF NOT EXISTS stream_session_id UUID 
    REFERENCES stream_sessions(id) ON DELETE SET NULL;

-- Step 5: Ensure coarse_geohash is NOT NULL (required for privacy)
-- First, set default value for existing NULL rows
-- Empty string indicates "no location specified" (distinct from NULL meaning "unknown")
-- This preserves privacy by requiring explicit location consent for all events
UPDATE events SET coarse_geohash = '' WHERE coarse_geohash IS NULL;

-- Then add NOT NULL constraint with empty string default
ALTER TABLE events ALTER COLUMN coarse_geohash SET NOT NULL;
ALTER TABLE events ALTER COLUMN coarse_geohash SET DEFAULT '';

-- Step 6: Add FTS generated column on title + tags
-- Note: Using GENERATED ALWAYS for computed tsvector
-- Combines title text with array_to_string for tags
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'events' AND column_name = 'title_tags_fts'
    ) THEN
        -- Only add if title column exists
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'events' AND column_name = 'title'
        ) THEN
            ALTER TABLE events ADD COLUMN title_tags_fts tsvector 
                GENERATED ALWAYS AS (
                    to_tsvector('english', 
                        COALESCE(title, '') || ' ' || COALESCE(array_to_string(tags, ' '), '')
                    )
                ) STORED;
        END IF;
    END IF;
END $$;

-- Step 7: Add indexes for query performance
-- GIN index on tags for array queries (exclude soft-deleted and cancelled events)
CREATE INDEX IF NOT EXISTS idx_events_tags ON events USING GIN(tags) 
    WHERE deleted_at IS NULL AND cancelled_at IS NULL;

-- GIN index for FTS queries on title + tags
CREATE INDEX IF NOT EXISTS idx_events_title_tags_fts ON events USING GIN(title_tags_fts);

-- Index on stream_session_id for join queries
CREATE INDEX IF NOT EXISTS idx_events_stream_session ON events(stream_session_id) 
    WHERE deleted_at IS NULL AND stream_session_id IS NOT NULL;

-- Index on status for filtering by event lifecycle (exclude soft-deleted and cancelled)
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status) 
    WHERE deleted_at IS NULL AND cancelled_at IS NULL;

-- Update table and column comments
COMMENT ON COLUMN events.title IS 'Event title, indexed for full-text search';
COMMENT ON COLUMN events.tags IS 'Categorization tags for discovery, indexed for FTS and array queries';
COMMENT ON COLUMN events.status IS 'Event lifecycle status (scheduled, live, ended, cancelled)';
COMMENT ON COLUMN events.stream_session_id IS 'Reference to active LiveKit stream session';
COMMENT ON COLUMN events.title_tags_fts IS 'Generated tsvector column for full-text search on title and tags';
