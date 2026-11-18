-- Back to the schema of 000034: the routes lose their Discord bot, the
-- Telegram bots their name and status, and the Discord bots table goes with
-- its tokens. discordd before this migration reads its one bot from
-- DISCORD_BOT_TOKEN again, which must then be set.
ALTER TABLE discord_telegram_routes
	DROP FOREIGN KEY fk_dtr_discord_bot,
	DROP KEY idx_dtr_discord_bot,
	DROP COLUMN discord_bot_id;

ALTER TABLE telegram_bots
	DROP COLUMN status_at,
	DROP COLUMN status_detail,
	DROP COLUMN status,
	DROP COLUMN name;

DROP TABLE discord_bots;
