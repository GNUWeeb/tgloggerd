-- Dummy files for the group page's Files tab
-- (internal/server/grouptabs_test.go), loaded after "sample" (its users
-- are the senders, its Assembly Lounge channel posts in a message).
-- Invented; nothing here comes from a real archive.
--
-- Ids, beside the sample's:
--   groups     -1001000000060 (Tabs Lab, supergroup, not exposed)
--   files      840001 - 840063
--   messages   group rows 6001 - 6063 (Tabs Lab, n = 1 - 63)
-- Tabs Lab's message n has row 6000 + n, message id (800000 + n) << 20
-- and date 1794931200 - 86400 + 60 * n; Dewi sends the odd n and Andi
-- the even n, but where a chat posts. By n:
--   1 - 52   photos (file 840000 + n, stored for odd n)
--   53 video   54 animation   55 audio   56 voice
--   57 document (notes.pdf)   58 sticker   59 video note
--   60 document posted as Tabs Lab itself (an anonymous admin)
--   61 photo posted as Assembly Lounge, which a member granted only
--      Tabs Lab does not see
--   62 text    63 a game with a file, a kind the tab does not list
-- The files tab of Tabs Lab thus lists 61 messages (n = 1 - 61).

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000060, 'supergroup', 'Tabs Lab', 'Files and polls.', 63,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title) VALUES
	(-1001000000060, 9299000001, 'administrator', '');

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 63)
SELECT 840000 + n, CONCAT('tabs-', 840000 + n), CONCAT('tabs-u-', 840000 + n),
	CASE WHEN n <= 52 OR n = 61 OR n = 63 THEN 'photo'
	     ELSE ELT(n - 52, 'video', 'animation', 'audio', 'voice', 'document', 'sticker',
			'video_note', 'document') END,
	1000 + n, n % 2, IF(n % 2 = 1, UNHEX(SHA2(CONCAT('tabs ', n), 256)), NULL),
	CASE WHEN n <= 52 OR n = 61 OR n = 63 THEN 'jpg'
	     ELSE ELT(n - 52, 'mp4', 'mp4', 'mp3', 'ogg', 'pdf', 'webp', 'mp4', 'txt') END,
	IF(n = 57, 'notes.pdf', ''), '2026-11-16 10:00:00', '2026-11-16 10:00:00'
FROM seq WHERE n <> 62;

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, file_id)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 63)
SELECT 6000 + n, -1001000000060, (800000 + n) << 20, IF(n % 2 = 1, 9299000001, 9299000004),
	1794931200 - 86400 + 60 * n,
	CASE WHEN n <= 52 OR n = 61 THEN 'photo' WHEN n = 62 THEN 'text' WHEN n = 63 THEN 'game'
	     ELSE ELT(n - 52, 'video', 'animation', 'audio', 'voice', 'document', 'sticker',
			'video_note', 'document') END,
	IF(n = 62, 'just text', NULL), IF(n = 62, NULL, 840000 + n)
FROM seq;

UPDATE telegram_group_messages SET sender_user_id = NULL, sender_chat_id = -1001000000060,
	author_signature = 'Admin' WHERE id = 6060;
UPDATE telegram_group_messages SET sender_user_id = NULL, sender_chat_id = -1001000000005,
	is_channel_post = 0 WHERE id = 6061;
