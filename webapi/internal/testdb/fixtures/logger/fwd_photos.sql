-- Dummy photos for the forwarding pages' chats (fwdphotos_test.go),
-- loaded after "sample", "forwarding" and "bots". Invented for tests.
--
-- Ids, beside theirs:
--   files  860002 (metadata only: Rust Programming's photo),
--          860003 (stored: Linux Kernel Indonesia's photo)
-- GNU/Weeb (880008) and Dewi Kusuma (880007) have stored photos in the
-- sample already.

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, hit_count, created_at, updated_at, accessed_at)
	VALUES
	(860002, 'AgACAgUAAxkBAAIfwdph02', 'AQADfwdph02', 'photo', 20480, 0,
	 NULL, 'jpg', '', 0,
	 '2026-10-02 10:00:00', '2026-10-02 10:00:00', '2026-10-02 10:00:00'),
	(860003, 'AgACAgUAAxkBAAIfwdph03', 'AQADfwdph03', 'photo', 20480, 1,
	 UNHEX(SHA2('fwd photos fixture 860003', 256)), 'jpg', '', 1,
	 '2026-10-03 10:00:00', '2026-10-03 10:00:00', '2026-11-17 09:00:00');

UPDATE telegram_groups SET photo_file_id = 860002 WHERE id = -1001000000003;
UPDATE telegram_groups SET photo_file_id = 860003 WHERE id = -1001000000002;
