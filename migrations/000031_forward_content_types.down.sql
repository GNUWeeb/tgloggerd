ALTER TABLE discord_telegram_routes
	DROP COLUMN fwd_document,
	DROP COLUMN fwd_video,
	DROP COLUMN fwd_photo,
	DROP COLUMN fwd_text,
	DROP COLUMN fwd_sticker;

ALTER TABLE telegram_discord_webhooks
	DROP COLUMN fwd_other,
	DROP COLUMN fwd_document,
	DROP COLUMN fwd_video,
	DROP COLUMN fwd_photo,
	DROP COLUMN fwd_text,
	DROP COLUMN fwd_sticker;
