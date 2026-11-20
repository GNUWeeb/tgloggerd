-- Discord bots in the database, one or more per account, instead of the one
-- token in discordd's environment (decisions/2026-11-18-ownership-and-bots.md).
--
-- discord_bots holds each bot's token beside telegram_bots, in plain text like
-- the Telegram tokens; the web treats both as write-only. discordd runs one
-- gateway connection per enabled bot and writes back what it sees: the status,
-- a line of detail and when, and, once logged in, the bot's own Discord user
-- id and username. A bot Discord refuses (a bad token, a missing privileged
-- intent) is disabled by discordd with its reason, never retried in a loop:
-- each IDENTIFY spends one of a daily budget whose exhaustion resets the
-- token. Enabling it again is one more try.
--
-- The ids start at 1000000000, so that a Discord bot's id never equals a
-- Telegram bot's: the web addresses both kinds as /api/bots/{id}.
CREATE TABLE discord_bots (
	id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	name          VARCHAR(64)     NOT NULL DEFAULT ''
	                              COMMENT 'The owner''s own label for the bot.',
	token         VARCHAR(255)    COLLATE utf8mb4_bin NOT NULL
	                              COMMENT 'Discord bot token. Write-only: never shown once saved.',
	enabled       TINYINT(1)      NOT NULL DEFAULT 1,
	status        ENUM('connecting','connected','intent_missing','token_refused','disabled')
	                              NOT NULL DEFAULT 'connecting'
	                              COMMENT 'As discordd last saw the bot.',
	status_detail VARCHAR(512)    DEFAULT NULL
	                              COMMENT 'discordd''s own words about the status.',
	status_at     DATETIME        DEFAULT NULL
	                              COMMENT 'When the status was last written.',
	bot_user_id   BIGINT UNSIGNED NOT NULL DEFAULT 0
	                              COMMENT 'The bot''s Discord user snowflake, learned at login.',
	username      VARCHAR(255)    NOT NULL DEFAULT '',

	created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP
	                              ON UPDATE CURRENT_TIMESTAMP,

	PRIMARY KEY (id),
	UNIQUE KEY uq_discord_bots_token (token),
	KEY idx_discord_bots_user (bot_user_id)
) ENGINE=InnoDB AUTO_INCREMENT=1000000000 DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Discord bot credentials discordd reads Discord channels with.';

-- The Telegram bots get a name and report their status as well, as discordd
-- sees them when it logs them in (intent_missing is Discord's only).
ALTER TABLE telegram_bots
	ADD COLUMN name          VARCHAR(64)  NOT NULL DEFAULT ''
	                         COMMENT 'The owner''s own label for the bot.' AFTER id,
	ADD COLUMN status        ENUM('connecting','connected','intent_missing','token_refused','disabled')
	                         NOT NULL DEFAULT 'connecting'
	                         COMMENT 'As discordd last saw the bot.' AFTER enabled,
	ADD COLUMN status_detail VARCHAR(512) DEFAULT NULL
	                         COMMENT 'discordd''s own words about the status.' AFTER status,
	ADD COLUMN status_at     DATETIME     DEFAULT NULL
	                         COMMENT 'When the status was last written.' AFTER status_detail;

UPDATE telegram_bots
	SET name = IF(username <> '', CONCAT('@', username), CONCAT('Bot ', id)),
	    status = IF(enabled, 'connecting', 'disabled'),
	    updated_at = updated_at;

-- A route names the Discord bot that reads its channel. NULL for the routes
-- made before this migration, until the import at deployment gives them the
-- bot made from the environment's token; discordd serves no route without one.
ALTER TABLE discord_telegram_routes
	ADD COLUMN discord_bot_id BIGINT UNSIGNED DEFAULT NULL
	           COMMENT 'FK to discord_bots.id: which bot reads the channel.'
	           AFTER telegram_bot_id,
	ADD KEY idx_dtr_discord_bot (discord_bot_id),
	ADD CONSTRAINT fk_dtr_discord_bot FOREIGN KEY (discord_bot_id)
		REFERENCES discord_bots (id);
