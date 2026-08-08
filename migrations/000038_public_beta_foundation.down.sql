DROP TABLE IF EXISTS event_location_grants;

ALTER TABLE events
    DROP COLUMN IF EXISTS location_access,
    DROP COLUMN IF EXISTS postponed_at,
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS publication_status,
    DROP COLUMN IF EXISTS public_slug;

ALTER TABLE tours
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS public_slug;

ALTER TABLE profiles
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS publication_status,
    DROP COLUMN IF EXISTS public_slug;

ALTER TABLE scenes
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS publication_status,
    DROP COLUMN IF EXISTS public_slug;

DROP TABLE IF EXISTS creator_access_requests;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS auth_magic_links;
DROP TABLE IF EXISTS auth_email_identities;

DROP INDEX IF EXISTS users_internal_did_unique;
ALTER TABLE users
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS onboarding_complete,
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS internal_did;
