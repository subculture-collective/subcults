-- Fail rollback rather than silently truncating real Tap checkpoint history.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM atproto_sync_observations
        WHERE source_event_id > 2147483647 OR source_event_id < -2147483648
    ) THEN
        RAISE EXCEPTION 'cannot narrow source_event_id while bigint values exist';
    END IF;
END $$;

ALTER TABLE atproto_sync_observations
    ALTER COLUMN source_event_id TYPE INTEGER;
