-- Dummy Discord bots for the forwarding tests, loaded after "sample" and
-- "forwarding" (whose Telegram bots and routes it uses). Invented: the
-- tokens are made up, shaped like Discord's, and open nothing.
--
-- Ids, beside the forwarding fixture's:
--   discord_bots              1000000001 (@gw_reader, connected),
--                             1000000002 (refused by Discord, disabled)
--   discord_telegram_routes   1 is read by 1000000001; 2 names no Discord
--                             bot (made before the bots were stored)
-- A test's own bots take ids from 1000000100.

INSERT INTO discord_bots (id, name, token, enabled, status, status_detail, status_at,
	bot_user_id, username) VALUES
	(1000000001, 'Main reader',
	 'MTAwMDAwMDAwMDAwMDAwMDAwMQ.GfixA.fixtureBotSecretDiscordOne', 1, 'connected', NULL,
	 '2026-11-18 01:00:00', 1000000000000000001, 'gw_reader'),
	(1000000002, 'Old reader',
	 'MTAwMDAwMDAwMDAwMDAwMDAwMg.GfixB.fixtureBotSecretDiscordTwo', 0, 'token_refused',
	 'Discord refused the token (close code 4004).', '2026-11-18 02:00:00', 0, '');

UPDATE discord_telegram_routes SET discord_bot_id = 1000000001 WHERE id = 1;
