-- Migration: Add allow_precise column to scenes and events tables
-- This column controls whether precise_point coordinates can be stored.
-- When false (default), precise_point must be NULL to protect user location privacy.

-- Add allow_precise column to scenes table (if it exists)
-- Default is FALSE to ensure privacy by default
ALTER TABLE scenes
ADD COLUMN IF NOT EXISTS allow_precise BOOLEAN NOT NULL DEFAULT FALSE;

-- Add allow_precise column to events table (if it exists)
-- Default is FALSE to ensure privacy by default
ALTER TABLE events
ADD COLUMN IF NOT EXISTS allow_precise BOOLEAN NOT NULL DEFAULT FALSE;

-- Add comment for documentation
COMMENT ON COLUMN scenes.allow_precise IS 'When false, precise_point must be NULL. Consent required for precise location storage.';
COMMENT ON COLUMN events.allow_precise IS 'When false, precise_point must be NULL. Consent required for precise location storage.';
