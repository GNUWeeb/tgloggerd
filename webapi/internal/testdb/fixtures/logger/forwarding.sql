-- Dummy forwarding configurations for the administration tests, loaded
-- after "sample" (its users and groups are referred to). Invented: the
-- webhook URLs and bot tokens are made up and open nothing.
--
-- Ids, beside the sample's:
--   telegram_discord_webhooks  1 (GNU/Weeb, sent 3 posts),
--                              2 (the private chat with Dewi Kusuma),
--                              3 (Rust Programming, disabled, text only)
--   discord_endpoints          1 (webhook 1's URL)
--   telegram_bots              1 (@gw_bridge_bot), 2 (not logged in yet,
--                              disabled)
--   discord_channels           710000000000000001 (#general)
--   discord_telegram_routes    1 (#general to GNU/Weeb, sent 2),
--                              2 (channel 710000000000000002 to Linux
--                              Kernel Indonesia, through bot 2, sent
--                              none)
-- A test's own rows take ids from 100.

-- The flags beyond migration 000031's are written as that migration's
-- successor leaves them (fwd_other's value; for Discord to Telegram audio
-- fwd_document's, animation fwd_photo's), so webhook 3 is still text only.
INSERT INTO telegram_discord_webhooks (id, telegram_chat_id, telegram_chat_type, webhook_url,
	enabled, fwd_sticker, fwd_text, fwd_photo, fwd_video, fwd_document, fwd_other,
	fwd_animation, fwd_video_note, fwd_voice, fwd_audio, fwd_poll, fwd_checklist, fwd_dice,
	fwd_game, fwd_location, fwd_contact, fwd_story, fwd_paid_media, fwd_invoice, fwd_service,
	created_at) VALUES
	(1, -1001000000001, 'group',
	 'https://discord.com/api/webhooks/1100000000000000001/fixtureSecretTokenOne', 1,
	 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, '2026-10-01 10:00:00'),
	(2, 9299000001, 'private',
	 'https://discord.com/api/webhooks/1100000000000000002/fixtureSecretTokenTwo', 1,
	 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, '2026-10-02 10:00:00'),
	(3, -1001000000003, 'group',
	 'https://discordapp.com/api/webhooks/1100000000000000003/fixtureSecretTokenThree', 0,
	 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '2026-10-03 10:00:00');

INSERT INTO discord_endpoints (id, webhook_url) VALUES
	(1, 'https://discord.com/api/webhooks/1100000000000000001/fixtureSecretTokenOne');

INSERT INTO telegram_discord_sent_messages (telegram_chat_id, telegram_message_id,
	endpoint_id, message_id, kind, created_at) VALUES
	(-1001000000001, 1048576, 1, '1200000000000000001', 'text', '2026-11-17 10:00:00'),
	(-1001000000001, 2097152, 1, '1200000000000000002', 'text', '2026-11-17 11:00:00'),
	(-1001000000001, 2097152, 1, '1200000000000000003', 'media', '2026-11-17 11:00:01');

INSERT INTO telegram_bots (id, token, bot_user_id, username, enabled) VALUES
	(1, '7000000001:fixtureBotSecretOne', 7000000001, 'gw_bridge_bot', 1),
	(2, '7000000002:fixtureBotSecretTwo', 0, '', 0);

INSERT INTO discord_channels (id, guild_id, name, type) VALUES
	(710000000000000001, 720000000000000001, 'general', 0);

INSERT INTO discord_telegram_routes (id, discord_channel_id, telegram_chat_id, telegram_bot_id,
	enabled, fwd_sticker, fwd_text, fwd_photo, fwd_video, fwd_document, fwd_audio,
	fwd_animation, fwd_voice, fwd_embed, fwd_poll, fwd_system, created_at) VALUES
	(1, 710000000000000001, -1001000000001, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		'2026-10-05 10:00:00'),
	(2, 710000000000000002, -1001000000002, 2, 1, 1, 1, 0, 0, 1, 1, 0, 0, 1, 0, 1,
		'2026-10-06 10:00:00');

INSERT INTO discord_telegram_sent_messages (discord_message_id, route_id, telegram_chat_id,
	telegram_message_id, created_at) VALUES
	(1300000000000000001, 1, -1001000000001, 9, '2026-11-17 12:00:00'),
	(1300000000000000002, 1, -1001000000001, 10, '2026-11-17 12:05:00');
