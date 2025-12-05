-- Migration: Enhance posts table for feed rendering and moderation
-- Adds: attachments JSONB, labels TEXT[], FTS column, scene_id/event_id constraint
-- Changes: scene_id becomes nullable, content renamed to text, attachment_url replaced

-- Step 1: Add new columns
ALTER TABLE posts ADD COLUMN IF NOT EXISTS attachments JSONB DEFAULT '[]'::jsonb;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS labels TEXT[] DEFAULT '{}';

-- Step 2: Rename content to text (if content exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'posts' AND column_name = 'content'
    ) THEN
        ALTER TABLE posts RENAME COLUMN content TO text;
    END IF;
END $$;

-- Step 3: Make scene_id nullable and update foreign key
-- First drop the existing constraint, then alter column, then add back constraint
ALTER TABLE posts ALTER COLUMN scene_id DROP NOT NULL;

-- Step 4: Migrate attachment_url data to attachments JSONB (if attachment_url exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'posts' AND column_name = 'attachment_url'
    ) THEN
        -- Migrate existing single attachment URLs to JSONB array format
        -- Uses 'legacy' type to indicate these are migrated from the old schema
        UPDATE posts 
        SET attachments = jsonb_build_array(jsonb_build_object('url', attachment_url, 'type', 'legacy'))
        WHERE attachment_url IS NOT NULL AND attachment_url != '';
        
        -- Drop the old column
        ALTER TABLE posts DROP COLUMN attachment_url;
    END IF;
END $$;

-- Step 5: Add FTS generated column
-- Note: Using GENERATED ALWAYS for computed tsvector
-- Requires 'text' column to exist (renamed from 'content' in Step 2)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'posts' AND column_name = 'text_fts'
    ) THEN
        -- Only add if text column exists (should always be true after Step 2)
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'posts' AND column_name = 'text'
        ) THEN
            ALTER TABLE posts ADD COLUMN text_fts tsvector 
                GENERATED ALWAYS AS (to_tsvector('english', COALESCE(text, ''))) STORED;
        END IF;
    END IF;
END $$;

-- Step 6: Add constraint - at least one of scene_id or event_id must be non-null
ALTER TABLE posts ADD CONSTRAINT chk_post_association 
    CHECK (scene_id IS NOT NULL OR event_id IS NOT NULL);

-- Step 7: Add indexes for query performance
-- Index on author_did (for user's posts)
CREATE INDEX IF NOT EXISTS idx_posts_author ON posts(author_did) WHERE deleted_at IS NULL;

-- Index on scene_id already exists as idx_posts_scene, but recreate with proper filtering
DROP INDEX IF EXISTS idx_posts_scene;
CREATE INDEX idx_posts_scene ON posts(scene_id) WHERE deleted_at IS NULL AND scene_id IS NOT NULL;

-- Index on event_id already exists as idx_posts_event, ensure proper filtering
DROP INDEX IF EXISTS idx_posts_event;
CREATE INDEX idx_posts_event ON posts(event_id) WHERE deleted_at IS NULL AND event_id IS NOT NULL;

-- GIN index for FTS queries
CREATE INDEX IF NOT EXISTS idx_posts_text_fts ON posts USING GIN(text_fts);

-- GIN index for labels array queries (moderation filtering)
CREATE INDEX IF NOT EXISTS idx_posts_labels ON posts USING GIN(labels);

-- Update table and column comments
COMMENT ON TABLE posts IS 'Content posts within scenes/events with attachments and moderation labels';
COMMENT ON COLUMN posts.text IS 'Post content text, indexed for full-text search';
COMMENT ON COLUMN posts.attachments IS 'JSONB array of attachment objects with url and type fields';
COMMENT ON COLUMN posts.labels IS 'Moderation labels (e.g., nsfw, spoiler)';
COMMENT ON COLUMN posts.text_fts IS 'Generated tsvector column for full-text search';
COMMENT ON CONSTRAINT chk_post_association ON posts IS 'Ensures post is associated with at least one of scene_id or event_id';
