-- Discord -> Telegram filters for the message kinds gwdiscord now tells
-- apart, beside those of migrations 000031 and 000048:
--
--   fwd_voice   a voice message's recording, sent as a voice note; it
--               followed fwd_audio as an audio/ogg attachment
--   fwd_embed   a message's embeds, rendered as text lines
--   fwd_poll    a poll's question and answers, rendered as text lines
--   fwd_system  a system message (a member joining, a pin, a boost, ...),
--               rendered as one line
--
-- The last three were posted under fwd_text, as a text-less message.
-- Each column is added with DEFAULT 1, then one UPDATE copies the flag
-- the kind followed until now, so no route changes what it forwards.
ALTER TABLE discord_telegram_routes
	ADD COLUMN fwd_voice  TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_animation,
	ADD COLUMN fwd_embed  TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_voice,
	ADD COLUMN fwd_poll   TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_embed,
	ADD COLUMN fwd_system TINYINT(1) NOT NULL DEFAULT 1 AFTER fwd_poll,
	ALGORITHM=INSTANT;

UPDATE discord_telegram_routes SET
	fwd_voice  = fwd_audio,
	fwd_embed  = fwd_text,
	fwd_poll   = fwd_text,
	fwd_system = fwd_text;
