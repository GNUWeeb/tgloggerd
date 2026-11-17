-- Dummy chats for the chat API's tests, loaded after "sample" (its
-- users and groups are referred to). Invented; nothing here comes from
-- a real archive.
--
-- Ids, beside the sample's:
--   users      9299000020
--   groups     -1001000000020 (Paging Lab, not exposed),
--              -1001000000021 (Open Lab, exposed)
--   files      870001 - 870006
--   messages   group rows 1001 - 1130 (Paging Lab, n = 1 - 130) and
--              2001 - 2004 (Open Lab); private rows 1001 - 1008 (with
--              9299000020)
-- Paging Lab's message n has message id (700000 + n) << 20 and date
-- 1794931200 - 86400 + 60 * n. Its features, by n:
--   10        edited twice (two earlier versions)
--   20        deleted
--   40 - 42   an album of three photos, the caption on 41
--   70 - 72   an album of three photos, which the newest window of 60
--             (n = 71 - 130) cuts
--   100       replies to 99 (same window)
--   101       replies to 5 (older windows)
--   102       replies to the sample's GNU/Weeb message row 2
--   103       replies to a message of a chat that was never logged
--   110       has a link preview with a stored photo
--   111       is forwarded from "Linus"
--   120       a photo whose bytes are not stored (small)
--   121       a video whose bytes are not stored (over any store cap)

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(870001, 'chats-fixture-870001', 'chats-u-870001', 'photo', 1000, 1,
	 UNHEX(SHA2('chats photo 870001', 256)), 'jpg', '', '2026-11-16 10:00:00', '2026-11-16 10:00:00'),
	(870002, 'chats-fixture-870002', 'chats-u-870002', 'photo', 1000, 1,
	 UNHEX(SHA2('chats photo 870002', 256)), 'jpg', '', '2026-11-16 10:00:00', '2026-11-16 10:00:00'),
	(870003, 'chats-fixture-870003', 'chats-u-870003', 'photo', 1000, 1,
	 UNHEX(SHA2('chats photo 870003', 256)), 'jpg', '', '2026-11-16 10:00:00', '2026-11-16 10:00:00'),
	(870004, 'chats-fixture-870004', 'chats-u-870004', 'photo', 2000, 1,
	 UNHEX(SHA2('chats preview 870004', 256)), 'jpg', '', '2026-11-16 10:00:00', '2026-11-16 10:00:00'),
	(870005, 'chats-fixture-870005', 'chats-u-870005', 'photo', 3000, 0,
	 NULL, 'jpg', '', '2026-11-16 10:00:00', '2026-11-16 10:00:00'),
	(870006, 'chats-fixture-870006', 'chats-u-870006', 'video', 2147483648, 0,
	 NULL, 'mp4', 'long-talk.mp4', '2026-11-16 10:00:00', '2026-11-16 10:00:00');

INSERT INTO telegram_users (id, first_name, last_name, type, msg_count, created_at, updated_at)
	VALUES (9299000020, 'Rina', 'Hartono', 'regular', 8, '2025-03-03 03:03:00',
	 '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES
	(-1001000000020, 'supergroup', 'Paging Lab', 'Enough messages to page across.', 130,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00'),
	(-1001000000021, 'supergroup', 'Open Lab', 'An exposed group.', 4,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_public_groups (group_id, note) VALUES (-1001000000021, 'tests');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 130)
SELECT 1000 + n, -1001000000020, (700000 + n) << 20,
	ELT(1 + n % 3, 9299000001, 9299000002, 9299000003),
	1794931200 - 86400 + 60 * n, 'text', CONCAT('paging message ', n)
FROM seq;

UPDATE telegram_group_messages SET edit_date = 1794931200 - 86400 + 60 * 10 + 30,
	text = 'paging message 10, third version' WHERE id = 1010;
INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, edit_date)
	VALUES (1010, 'text', 'paging message 10', 0),
	(1010, 'text', 'paging message 10, second version', 1794931200 - 86400 + 60 * 10 + 10);

UPDATE telegram_group_messages SET deleted_at = '2026-11-17 12:00:00' WHERE id = 1020;

UPDATE telegram_group_messages SET content_type = 'photo', media_album_id = 7001, text = NULL,
	file_id = 870001 WHERE id = 1040;
UPDATE telegram_group_messages SET content_type = 'photo', media_album_id = 7001,
	text = 'album caption', file_id = 870002 WHERE id = 1041;
UPDATE telegram_group_messages SET content_type = 'photo', media_album_id = 7001, text = NULL,
	file_id = 870003 WHERE id = 1042;
UPDATE telegram_group_messages SET content_type = 'photo', media_album_id = 7002, text = NULL,
	file_id = 870001, sender_user_id = 9299000001 WHERE id IN (1070, 1071, 1072);
UPDATE telegram_group_messages SET text = 'edge album caption' WHERE id = 1072;

UPDATE telegram_group_messages SET reply_to_chat_id = -1001000000020,
	reply_to_msg_id = (700000 + 99) << 20, reply_to_id = 1099 WHERE id = 1100;
UPDATE telegram_group_messages SET reply_to_chat_id = -1001000000020,
	reply_to_msg_id = (700000 + 5) << 20, reply_to_id = 1005 WHERE id = 1101;
UPDATE telegram_group_messages SET reply_to_chat_id = -1001000000001,
	reply_to_msg_id = 410002 << 20, reply_to_id = 2 WHERE id = 1102;
UPDATE telegram_group_messages SET reply_to_chat_id = -1001000000099,
	reply_to_msg_id = 12345 << 20 WHERE id = 1103;

INSERT INTO telegram_link_previews (group_message_id, url, display_url, site_name, title,
	description, type, photo_file_id) VALUES
	(1110, 'https://example.org/kernel', 'example.org/kernel', 'Example', 'Kernel notes',
	 'Notes on the kernel.', 'article', 870004);

UPDATE telegram_group_messages SET is_forwarded = 1 WHERE id = 1111;
INSERT INTO telegram_group_message_fwd_info (group_message_id, origin_type, origin_sender_name)
	VALUES (1111, 'hidden_user', 'Linus');

UPDATE telegram_group_messages SET content_type = 'photo', text = NULL, file_id = 870005
	WHERE id = 1120;
UPDATE telegram_group_messages SET content_type = 'video', text = 'the long talk',
	file_id = 870006 WHERE id = 1121;

-- Open Lab: 2004 replies to Paging Lab's message 100, which signed-out
-- visitors may not see.
INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, reply_to_chat_id, reply_to_msg_id, reply_to_id) VALUES
	(2001, -1001000000021, 800001 << 20, 9299000003, 1794931200 - 600, 'text', 'open hello',
	 NULL, NULL, NULL),
	(2002, -1001000000021, 800002 << 20, 9299000001, 1794931200 - 500, 'text', 'open reply',
	 -1001000000021, 800001 << 20, 2001),
	(2003, -1001000000021, 800003 << 20, 9299000002, 1794931200 - 400, 'text', 'open three',
	 NULL, NULL, NULL),
	(2004, -1001000000021, 800004 << 20, 9299000002, 1794931200 - 300, 'text',
	 'quoting the lab', -1001000000020, (700000 + 100) << 20, 1100);

-- A private chat with Rina: outgoing messages are the logging
-- account's; 1005 and 1006 are an album; 1008 replies to 1001.
INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing, date,
	content_type, text, file_id, media_album_id, reply_to_chat_id, reply_to_msg_id,
	reply_to_id) VALUES
	(1001, 9299000020, 900001 << 20, 9299000020, 0, 1794931200 - 3600, 'text', 'hai',
	 NULL, NULL, NULL, NULL, NULL),
	(1002, 9299000020, 900002 << 20, NULL, 1, 1794931200 - 3500, 'text', 'halo rina',
	 NULL, NULL, NULL, NULL, NULL),
	(1003, 9299000020, 900003 << 20, 9299000020, 0, 1794931200 - 3400, 'text', 'three',
	 NULL, NULL, NULL, NULL, NULL),
	(1004, 9299000020, 900004 << 20, NULL, 1, 1794931200 - 3300, 'text', 'four',
	 NULL, NULL, NULL, NULL, NULL),
	(1005, 9299000020, 900005 << 20, 9299000020, 0, 1794931200 - 3200, 'photo', 'holiday',
	 870002, 7003, NULL, NULL, NULL),
	(1006, 9299000020, 900006 << 20, 9299000020, 0, 1794931200 - 3200, 'photo', NULL,
	 870003, 7003, NULL, NULL, NULL),
	(1007, 9299000020, 900007 << 20, NULL, 1, 1794931200 - 3000, 'text', 'nice',
	 NULL, NULL, NULL, NULL, NULL),
	(1008, 9299000020, 900008 << 20, NULL, 1, 1794931200 - 2900, 'text', 'replying',
	 NULL, NULL, 9299000020, 900001 << 20, 1001);
