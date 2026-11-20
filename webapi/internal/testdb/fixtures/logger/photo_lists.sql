-- Dummy photo lists whose current photo the history lacks, with photos
-- the store no longer holds (photolists_test.go), loaded after
-- "sample". Invented for tests.
--
-- Ids, beside the sample's:
--   users    9299410001 (Foto Baru)
--   groups   -1001000041001 (Foto Grup)
--   files    851001 - 851005
--   history  row ids 1101 - 1199 in telegram_user_hist_profile_photo
--            and telegram_group_hist_photo
--
-- Foto Baru's current photo (851003, stored) has no history row; the
-- history's newest row (1102, 851002, evicted: metadata only) is later
-- than when the current photo's file was recorded. 1101 (851001) is
-- stored.
-- Foto Grup's current photo (851004, evicted) has no history row
-- either; its history is 1103 (851005, evicted) and 1104 (851001).

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, hit_count, created_at, updated_at, accessed_at)
	VALUES
	(851001, 'AgACAgUAAxkBAAIplist01', 'AQADplist01', 'photo', 30720, 1,
	 UNHEX(SHA2('photo lists fixture 851001', 256)), 'jpg', '', 1,
	 '2026-08-01 10:00:00', '2026-08-01 10:00:00', '2026-11-17 09:00:00'),
	(851002, 'AgACAgUAAxkBAAIplist02', 'AQADplist02', 'photo', 30720, 0,
	 NULL, 'jpg', '', 0,
	 '2026-09-01 16:39:11', '2026-09-01 16:39:11', '2026-09-01 16:39:11'),
	(851003, 'AgACAgUAAxkBAAIplist03', 'AQADplist03', 'photo', 30720, 1,
	 UNHEX(SHA2('photo lists fixture 851003', 256)), 'jpg', '', 1,
	 '2026-09-01 16:38:00', '2026-09-01 16:38:00', '2026-11-17 09:00:00'),
	(851004, 'AgACAgUAAxkBAAIplist04', 'AQADplist04', 'photo', 30720, 0,
	 NULL, 'jpg', '', 0,
	 '2026-10-05 10:00:00', '2026-10-05 10:00:00', '2026-10-05 10:00:00'),
	(851005, 'AgACAgUAAxkBAAIplist05', 'AQADplist05', 'photo', 30720, 0,
	 NULL, 'jpg', '', 0,
	 '2026-06-01 10:00:00', '2026-06-01 10:00:00', '2026-06-01 10:00:00');

INSERT INTO telegram_users (id, first_name, last_name, type, profile_photo_file_id,
	msg_count, created_at, updated_at) VALUES
	(9299410001, 'Foto', 'Baru', 'regular', 851003, 0,
	 '2026-01-01 08:00:00', '2026-11-10 10:00:00');

INSERT INTO telegram_user_hist_profile_photo (id, user_id, file_id, created_at) VALUES
	(1101, 9299410001, 851001, '2026-08-01 10:00:00'),
	(1102, 9299410001, 851002, '2026-09-01 16:39:11');

INSERT INTO telegram_groups (id, type, title, description, photo_file_id, msg_count,
	created_at, updated_at) VALUES
	(-1001000041001, 'supergroup', 'Foto Grup', 'Photos only.', 851004, 0,
	 '2024-01-01 09:00:00', '2026-11-10 10:00:00');

INSERT INTO telegram_group_hist_photo (id, group_id, file_id, created_at) VALUES
	(1103, -1001000041001, 851005, '2026-06-01 10:00:00'),
	(1104, -1001000041001, 851001, '2026-08-01 10:00:00');
