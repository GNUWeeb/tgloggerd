ALTER TABLE discord_telegram_routes
	DROP COLUMN fwd_system,
	DROP COLUMN fwd_poll,
	DROP COLUMN fwd_embed,
	DROP COLUMN fwd_voice;
