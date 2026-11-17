-- Table-level privileges for the web's read-only user on logger tables.
--
-- MySQL refuses a table-level GRANT on a table that does not exist yet
-- (ERROR 1146) unless the grant includes CREATE, so these cannot run in
-- docker/mysql/init/10-web-users.sh: on a fresh data directory the daemon's
-- migrations have not created the tables at that point. run.sh renders this
-- file (@TG_DB_NAME@, @WEB_DB_RO_USER@) and applies it as MySQL root after
-- every `migrate up` of the daemon. GRANT only adds privileges, so a repeat
-- run changes nothing. Add a table here in the same change as the migration
-- that creates it.

-- The web app manages the Discord webhook integrations, which live in the
-- logger's schema; grant it write access to just that one table (the read-only
-- boundary otherwise holds).
GRANT INSERT, UPDATE, DELETE ON `@TG_DB_NAME@`.telegram_discord_webhooks TO '@WEB_DB_RO_USER@'@'%';

-- The same for the opposite direction: /routes manages the Discord -> Telegram
-- forwarding routes and the bot credentials they send through. telegram_bots
-- holds bot TOKENS, so this grant is the reason that table is never SELECTed
-- for display -- see web/src/dao/Routes.hpp.
GRANT INSERT, UPDATE, DELETE ON `@TG_DB_NAME@`.discord_telegram_routes TO '@WEB_DB_RO_USER@'@'%';
GRANT INSERT, UPDATE, DELETE ON `@TG_DB_NAME@`.telegram_bots TO '@WEB_DB_RO_USER@'@'%';

-- The MCP exposure allowlist, managed from the admin UI. Rows are added and
-- removed, never updated, so UPDATE is deliberately not granted -- this table
-- decides what leaves the archive, and a narrower grant is worth the pedantry.
GRANT INSERT, DELETE ON `@TG_DB_NAME@`.telegram_public_groups TO '@WEB_DB_RO_USER@'@'%';
