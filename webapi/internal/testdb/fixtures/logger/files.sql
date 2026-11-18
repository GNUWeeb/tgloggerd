-- Dummy files for the tests of fetching unstored files from the chat
-- API and the files list (internal/server/files_test.go), loaded after
-- "sample" (its users are the senders). Invented; nothing here comes
-- from a real archive.
--
-- Ids, beside the sample's:
--   groups     -1001000000030 (Fetch Lab, exposed)
--   files      860001 - 860005
--   messages   group rows 3001 - 3005, Telegram message ids
--              (720000 + n) << 20 for row 3000 + n
-- The tests run with a per-file store cap of 1 MiB, so by size:
--   860001  photo, stored (its blob is written by the test)
--   860002  photo, metadata only (no sha256, never downloaded), small
--   860003  video, metadata only, big
--   860004  document, evicted (sha256 kept, bytes gone), small
--   860005  audio, stored

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, hit_count, created_at, updated_at, accessed_at)
	VALUES
	(860001, 'AgACAgUAAxkBAAIfetch01', 'AQADfetch01', 'photo', 2048, 1,
	 UNHEX(SHA2('files fixture 860001', 256)), 'jpg', '', 3,
	 '2026-11-10 10:00:00', '2026-11-10 10:00:00', '2026-11-17 09:00:00'),
	(860002, 'AgACAgUAAxkBAAIfetch02', 'AQADfetch02', 'photo', 5000, 0,
	 NULL, 'jpg', '', 0,
	 '2026-11-11 10:00:00', '2026-11-11 10:00:00', '2026-11-11 10:00:00'),
	(860003, 'BAACAgUAAxkBAAIfetch03', 'AgADfetch03', 'video', 2147483648, 0,
	 NULL, 'mp4', 'kernel-recipes.mp4', 0,
	 '2026-11-12 10:00:00', '2026-11-12 10:00:00', '2026-11-12 10:00:00'),
	(860004, 'BQACAgUAAxkBAAIfetch04', 'AgADfetch04', 'document', 40000, 0,
	 UNHEX(SHA2('files fixture 860004', 256)), 'pdf', 'evicted-notes.pdf', 7,
	 '2026-11-13 10:00:00', '2026-11-16 10:00:00', '2026-11-14 10:00:00'),
	(860005, 'CQACAgUAAxkBAAIfetch05', 'AgADfetch05', 'audio', 300000, 1,
	 UNHEX(SHA2('files fixture 860005', 256)), 'ogg', 'theme.ogg', 1,
	 '2026-11-14 10:00:00', '2026-11-14 10:00:00', '2026-11-16 09:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000030, 'supergroup', 'Fetch Lab', 'Files stored and not.', 5,
	 '2025-07-07 07:07:00', '2026-11-17 15:00:00');

INSERT INTO telegram_public_groups (group_id, note) VALUES (-1001000000030, 'tests');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, file_id) VALUES
	(3001, -1001000000030, 720001 << 20, 9299000001, 1794931200 - 500, 'photo', NULL, 860001),
	(3002, -1001000000030, 720002 << 20, 9299000002, 1794931200 - 400, 'photo', NULL, 860002),
	(3003, -1001000000030, 720003 << 20, 9299000001, 1794931200 - 300, 'video', NULL, 860003),
	(3004, -1001000000030, 720004 << 20, 9299000003, 1794931200 - 200, 'document', NULL,
	 860004),
	(3005, -1001000000030, 720005 << 20, 9299000002, 1794931200 - 100, 'audio', NULL, 860005);
