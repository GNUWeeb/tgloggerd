-- Per-route content-type filters for both forwarding directions.
--
-- Each route gains one flag per kind of content it may carry; a part of a
-- message is forwarded only when its flag is set. A message is filtered part
-- by part: its text (or caption) follows fwd_text and each media item follows
-- the flag of its own type, so turning photos off still lets a caption through.
--
-- Every flag defaults to 1, so existing routes keep forwarding everything.
--
-- Telegram -> Discord also has fwd_other, for service (system) messages and
-- every content type without a flag of its own (voice, audio, animation, ...).
-- Discord -> Telegram has no such class: an attachment is a photo (image/*),
-- a video (video/*) or else a document.
ALTER TABLE telegram_discord_webhooks
	ADD COLUMN fwd_sticker  TINYINT(1) NOT NULL DEFAULT 1 AFTER enabled,
	ADD COLUMN fwd_text     TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_sticker,
	ADD COLUMN fwd_photo    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_text,
	ADD COLUMN fwd_video    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_photo,
	ADD COLUMN fwd_document TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_video,
	ADD COLUMN fwd_other    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_document;

ALTER TABLE discord_telegram_routes
	ADD COLUMN fwd_sticker  TINYINT(1) NOT NULL DEFAULT 1 AFTER enabled,
	ADD COLUMN fwd_text     TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_sticker,
	ADD COLUMN fwd_photo    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_text,
	ADD COLUMN fwd_video    TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_photo,
	ADD COLUMN fwd_document TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_video;
