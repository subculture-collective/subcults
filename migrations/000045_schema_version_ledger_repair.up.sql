-- Migrations 43 and 44 were applied through golang-migrate but did not append
-- their versions to the application compatibility ledger. Record the current
-- compatible schema without rewriting migrations that may already be applied.
INSERT INTO schema_version(version, description)
SELECT 45, 'repair application schema-version ledger after AT Protocol migrations'
WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 45);
