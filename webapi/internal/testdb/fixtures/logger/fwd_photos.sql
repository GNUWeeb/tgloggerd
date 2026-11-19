-- Dummy photos for the forwarding pages, the pinned groups and the bots
-- (fwdphotos_test.go), loaded after "sample", "forwarding" and "bots".
-- Invented for tests.
--
-- Ids, beside theirs:
--   files  860001 (stored: @gw_bridge_bot's profile photo),
--          860002 (metadata only: Rust Programming's photo),
--          860003 (stored: Linux Kernel Indonesia's photo, and the
--          profile photo of a Telegram user whose id is the Discord
--          bot gw_reader's user id, which must never show for it)
--   users  7000000001 (@gw_bridge_bot, telegram bot 1),
--          1000000000000000001 (an id equal to discord bot 1000000001's
--          user id)
-- GNU/Weeb (880008) and Dewi Kusuma (880007) have stored photos in the
-- sample already.

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, hit_count, created_at, updated_at, accessed_at)
	VALUES
	(860001, 'AgACAgUAAxkBAAIfwdph01', 'AQADfwdph01', 'photo', 20480, 1,
	 UNHEX(SHA2('fwd photos fixture 860001', 256)), 'jpg', '', 1,
	 '2026-10-01 10:00:00', '2026-10-01 10:00:00', '2026-11-17 09:00:00'),
	(860002, 'AgACAgUAAxkBAAIfwdph02', 'AQADfwdph02', 'photo', 20480, 0,
	 NULL, 'jpg', '', 0,
	 '2026-10-02 10:00:00', '2026-10-02 10:00:00', '2026-10-02 10:00:00'),
	(860003, 'AgACAgUAAxkBAAIfwdph03', 'AQADfwdph03', 'photo', 20480, 1,
	 UNHEX(SHA2('fwd photos fixture 860003', 256)), 'jpg', '', 1,
	 '2026-10-03 10:00:00', '2026-10-03 10:00:00', '2026-11-17 09:00:00');

INSERT INTO telegram_users (id, first_name, last_name, type, profile_photo_file_id,
	msg_count, created_at, updated_at) VALUES
	(7000000001, 'GW Bridge', '', 'bot', 860001, 0,
	 '2026-10-01 10:00:00', '2026-11-10 10:00:00'),
	(1000000000000000001, 'Not', 'Discord', 'regular', 860003, 0,
	 '2026-10-01 10:00:00', '2026-11-10 10:00:00');

UPDATE telegram_groups SET photo_file_id = 860002 WHERE id = -1001000000003;
UPDATE telegram_groups SET photo_file_id = 860003 WHERE id = -1001000000002;
