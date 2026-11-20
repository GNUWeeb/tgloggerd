-- More per-route content-type filters for both forwarding directions
-- (decision 2026-11-20-fwd-more-types), beside those of migration 000031.
--
-- Telegram -> Discord: until now every one of these kinds followed
-- fwd_other. A kind's summary line (a location, a contact, a poll, ...)
-- follows its kind's flag; fwd_other stays the catch-all for rich
-- messages, calls, giveaways and every kind without a flag of its own.
--
-- Discord -> Telegram: fwd_audio covers audio/* attachments, which
-- followed fwd_document; fwd_animation covers image/gif attachments and
-- animated stickers, which followed fwd_photo.
--
-- Each column is added with DEFAULT 1, then one UPDATE per table copies
-- the flag the kind followed until now, so no route changes what it
-- forwards. The tables are small, but ALGORITHM=INSTANT still makes a
-- server that cannot add the columns in place fail the statement rather
-- than copy the table.
ALTER TABLE telegram_discord_webhooks
	ADD COLUMN fwd_animation  TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_other,
	ADD COLUMN fwd_video_note TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_animation,
	ADD COLUMN fwd_voice      TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_video_note,
	ADD COLUMN fwd_audio      TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_voice,
	ADD COLUMN fwd_poll       TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_audio,
	ADD COLUMN fwd_checklist  TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_poll,
	ADD COLUMN fwd_dice       TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_checklist,
	ADD COLUMN fwd_game       TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_dice,
	ADD COLUMN fwd_location   TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_game,
	ADD COLUMN fwd_contact    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_location,
	ADD COLUMN fwd_story      TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_contact,
	ADD COLUMN fwd_paid_media TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_story,
	ADD COLUMN fwd_invoice    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_paid_media,
	ADD COLUMN fwd_service    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_invoice,
	ALGORITHM=INSTANT;

ALTER TABLE discord_telegram_routes
	ADD COLUMN fwd_audio     TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_document,
	ADD COLUMN fwd_animation TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_audio,
	ALGORITHM=INSTANT;

UPDATE telegram_discord_webhooks SET
	fwd_animation  = fwd_other,
	fwd_video_note = fwd_other,
	fwd_voice      = fwd_other,
	fwd_audio      = fwd_other,
	fwd_poll       = fwd_other,
	fwd_checklist  = fwd_other,
	fwd_dice       = fwd_other,
	fwd_game       = fwd_other,
	fwd_location   = fwd_other,
	fwd_contact    = fwd_other,
	fwd_story      = fwd_other,
	fwd_paid_media = fwd_other,
	fwd_invoice    = fwd_other,
	fwd_service    = fwd_other;

UPDATE discord_telegram_routes SET
	fwd_audio     = fwd_document,
	fwd_animation = fwd_photo;
