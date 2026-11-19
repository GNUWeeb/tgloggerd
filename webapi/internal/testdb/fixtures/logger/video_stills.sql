-- Dummy videos for the pictures they show before they play (the video
-- stills tests), loaded after "sample" (its users are the senders).
-- Invented; nothing here comes from a real archive.
--
-- Ids, beside the sample's:
--   groups     -1001000000050 (Still Lab, supergroup, not exposed),
--              -1001000000051 (Open Stills, supergroup, for a member)
--   files      890001 - 890008 the videos, 890101 - 890108 their pictures
--   messages   group rows 5001 - 5009 (Still Lab, n = 1 - 9), 5021 -
--              5022 (Open Stills)
-- Still Lab's message n has message id (700000 + n) << 20 and date
-- 1794931200 - 3600 + 60 * n. By n:
--   1  a video with a stored thumbnail (890101)
--   2  an animation with a stored thumbnail (890102)
--   3  a video note with a stored thumbnail (890103)
--   4  a .webm sticker with a stored thumbnail (890104, WebP)
--   5  a video with a stored cover (890105) and a stored thumbnail
--      (890106): the cover wins
--   6  a video with neither, only a minithumbnail
--   7  a video whose thumbnail (890107) is not stored
--   8  a video logged before thumbnails were stored: none
--   9  the same video as 8 sent again, with a stored thumbnail (890108)
-- Open Stills sends 1 again without its pictures (5021), and 2 again
-- with its thumbnail (5022), for a member granted only that group.

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 8)
SELECT 890000 + n, CONCAT('still-', 890000 + n), CONCAT('still-u-', 890000 + n),
	ELT(n, 'video', 'animation', 'video_note', 'sticker', 'video', 'video', 'video', 'video'),
	500000, 1, UNHEX(SHA2(CONCAT('still ', 890000 + n), 256)),
	IF(n = 4, 'webm', 'mp4'), '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'
FROM seq;

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 8)
SELECT 890100 + n, CONCAT('still-', 890100 + n), CONCAT('still-u-', 890100 + n),
	IF(n = 5, 'photo', 'thumbnail'), 9000, IF(n = 7, 0, 1),
	IF(n = 7, NULL, UNHEX(SHA2(CONCAT('still ', 890100 + n), 256))),
	IF(n = 4, 'webp', 'jpg'), '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'
FROM seq;

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000050, 'supergroup', 'Still Lab', 'Videos and their stills.', 9,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00'),
	(-1001000000051, 'supergroup', 'Open Stills', 'Videos sent again.', 2,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, file_id, tdlib_type, content_extra)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 9)
SELECT 5000 + n, -1001000000050, (700000 + n) << 20, 9299000001,
	1794931200 - 3600 + 60 * n,
	ELT(n, 'video', 'animation', 'video_note', 'sticker', 'video', 'video', 'video', 'video',
	 'video'),
	NULL, 890000 + LEAST(n, 8),
	ELT(n, 'messageVideo', 'messageAnimation', 'messageVideoNote', 'messageSticker',
	 'messageVideo', 'messageVideo', 'messageVideo', 'messageVideo', 'messageVideo'),
	IF(n = 4, JSON_OBJECT('format', 'webm', 'emoji', 'ok'),
	 JSON_OBJECT('width', 640, 'height', 360, 'duration', 5, 'minithumbnail',
	 JSON_OBJECT('width', 40, 'height', 22, 'data', CONCAT('MINI', n, 'abc'))))
FROM seq;

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, file_id) VALUES
	(5021, -1001000000051, 710001 << 20, 9299000001, 1794931200 - 600, 'video', 890001),
	(5022, -1001000000051, 710002 << 20, 9299000001, 1794931200 - 540, 'animation', 890002);

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(5001, 'thumbnail', 0, 890101),
	(5002, 'thumbnail', 0, 890102),
	(5003, 'thumbnail', 0, 890103),
	(5004, 'thumbnail', 0, 890104),
	(5005, 'cover', 0, 890105),
	(5005, 'thumbnail', 0, 890106),
	(5007, 'thumbnail', 0, 890107),
	(5009, 'thumbnail', 0, 890108),
	(5022, 'thumbnail', 0, 890102);
