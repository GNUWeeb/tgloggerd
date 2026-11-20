-- Dummy photo history for the user and group pages' photo carousel
-- (GET /api/users/{id}/photos, /api/groups/{id}/photos), loaded after
-- "sample". Invented for tests.
--
-- Ids, beside the sample's:
--   users      9299400001 (Foto Banyak), 9299400002 (Foto Lain)
--   files      850001 - 850003
--   history    row ids 1001 - 1099 in telegram_user_hist_profile_photo
--              and telegram_group_hist_photo
--
-- Foto Banyak's photos, newest first: 1005 (850003, a later photo
-- recorded at the same second as 1004), 1004 (850001 set again, the
-- current photo), 1003 (850002, metadata only: the tests have no
-- daemon, so it cannot be served), 1002 (removed), 1001 (850001).
-- Foto Lain has one photo (1006), which must not show for Foto Banyak.
-- GNU/Weeb's (sample) photos, newest first: 1003 (880008, the current
-- photo), 1002 (removed), 1001 (850003), 1004 (850002, cannot be
-- served). Linux Kernel Indonesia (sample) has one photo (1005).

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, hit_count, created_at, updated_at, accessed_at)
	VALUES
	(850001, 'AgACAgUAAxkBAAIphoto01', 'AQADphoto01', 'photo', 30720, 1,
	 UNHEX(SHA2('photos fixture 850001', 256)), 'jpg', '', 1,
	 '2026-03-01 10:00:00', '2026-03-01 10:00:00', '2026-11-17 09:00:00'),
	(850002, 'AgACAgUAAxkBAAIphoto02', 'AQADphoto02', 'photo', 30720, 0,
	 NULL, 'jpg', '', 0,
	 '2026-05-01 10:00:00', '2026-05-01 10:00:00', '2026-05-01 10:00:00'),
	(850003, 'AgACAgUAAxkBAAIphoto03', 'AQADphoto03', 'photo', 30720, 1,
	 UNHEX(SHA2('photos fixture 850003', 256)), 'jpg', '', 1,
	 '2026-07-01 10:00:00', '2026-07-01 10:00:00', '2026-11-17 09:00:00');

INSERT INTO telegram_users (id, first_name, last_name, type, profile_photo_file_id,
	msg_count, created_at, updated_at) VALUES
	(9299400001, 'Foto', 'Banyak', 'regular', 850001, 0,
	 '2026-01-01 08:00:00', '2026-11-10 10:00:00'),
	(9299400002, 'Foto', 'Lain', 'regular', 850003, 0,
	 '2026-01-01 08:00:00', '2026-11-10 10:00:00');

INSERT INTO telegram_user_hist_profile_photo (id, user_id, file_id, created_at) VALUES
	(1001, 9299400001, 850001, '2026-03-01 10:00:00'),
	(1002, 9299400001, NULL, '2026-04-01 10:00:00'),
	(1003, 9299400001, 850002, '2026-05-01 10:00:00'),
	(1004, 9299400001, 850001, '2026-07-01 10:00:00'),
	(1005, 9299400001, 850003, '2026-07-01 10:00:00'),
	(1006, 9299400002, 850003, '2026-07-02 10:00:00');

INSERT INTO telegram_group_hist_photo (id, group_id, file_id, created_at) VALUES
	(1001, -1001000000001, 850003, '2026-07-01 10:00:00'),
	(1002, -1001000000001, NULL, '2026-08-01 10:00:00'),
	(1003, -1001000000001, 880008, '2026-09-01 10:00:00'),
	(1004, -1001000000001, 850002, '2026-02-01 10:00:00'),
	(1005, -1001000000002, 850001, '2026-09-02 10:00:00');
