-- Venues are editable Studio drafts and require the same optimistic
-- concurrency boundary as other public touring entities.
ALTER TABLE venues ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
