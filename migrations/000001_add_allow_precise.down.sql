-- Rollback: Remove allow_precise column from scenes and events tables

-- Remove allow_precise column from events table
ALTER TABLE events DROP COLUMN IF EXISTS allow_precise;

-- Remove allow_precise column from scenes table
ALTER TABLE scenes DROP COLUMN IF EXISTS allow_precise;
