-- Dummy chats for the chat view's media and details (the parity tests
-- of the chat and message pages), loaded after "sample" (its users are
-- the senders). Invented; nothing here comes from a real archive.
--
-- Ids, beside the sample's and the "chats" fixture's:
--   users      9299000030 (Tono, a private chat)
--   groups     -1001000000030 (Media Lab, supergroup, not exposed),
--              -1001000000031 (Channel News, channel)
--   files      860001 - 860014
--   messages   group rows 3001 - 3018 (Media Lab, n = 1 - 18) and 3101
--              (Channel News); private rows 3001 - 3002 (with Tono)
-- Media Lab's message n has message id (600000 + n) << 20 and date
-- 1794931200 - 7200 + 60 * n. Rows logged since migration 000034 carry
-- content_extra; rows 2 and 5 (and the private 3002) are older rows
-- without it. By n:
--   1         a video with width, height, duration, mime, minithumbnail
--             and a cover photo (telegram_message_files)
--   2         a video without content_extra, not stored
--   3         an animation with a minithumbnail
--   4         a .webp sticker with its emoji
--   5         a .webm sticker without content_extra (format from ext)
--   6         a .tgs sticker
--   7         a voice note with a waveform (values 1 2 3 4 5 6 7 31)
--   8         an audio file with title and performer
--   9 - 11    an album: a photo, a video (deleted), a document with
--             the caption
--   12        a photo, edited from a text and then a video
--   13        forwarded from the user Sari
--   14        forwarded from the channel Channel News, its post 77
--   15        posted as Channel News, signed "Editor"
--   16        replies to 10 (deleted)
--   17        replies to a message that was never logged
--   18        a service message (chat_add_members) without text
--   19        a round video note (a kind added by migration 000034)
-- Its senders are Dewi, Bima, Sari and Andi; Dewi and Putri are its
-- admins, so the archive has seen 5 members.

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(860001, 'parity-860001', 'parity-u-860001', 'video', 500000, 1,
	 UNHEX(SHA2('parity 860001', 256)), 'mp4', 'clip.mp4', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860002, 'parity-860002', 'parity-u-860002', 'video', 900000, 0,
	 NULL, 'mp4', 'old-clip.mp4', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860003, 'parity-860003', 'parity-u-860003', 'animation', 80000, 1,
	 UNHEX(SHA2('parity 860003', 256)), 'mp4', 'loop.mp4', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860004, 'parity-860004', 'parity-u-860004', 'sticker', 20000, 1,
	 UNHEX(SHA2('parity 860004', 256)), 'webp', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860005, 'parity-860005', 'parity-u-860005', 'sticker', 30000, 1,
	 UNHEX(SHA2('parity 860005', 256)), 'webm', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860006, 'parity-860006', 'parity-u-860006', 'sticker', 25000, 1,
	 UNHEX(SHA2('parity 860006', 256)), 'tgs', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860007, 'parity-860007', 'parity-u-860007', 'voice', 12000, 1,
	 UNHEX(SHA2('parity 860007', 256)), 'oga', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860008, 'parity-860008', 'parity-u-860008', 'audio', 4000000, 1,
	 UNHEX(SHA2('parity 860008', 256)), 'mp3', 'song.mp3', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860009, 'parity-860009', 'parity-u-860009', 'photo', 60000, 1,
	 UNHEX(SHA2('parity 860009', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860010, 'parity-860010', 'parity-u-860010', 'video', 700000, 1,
	 UNHEX(SHA2('parity 860010', 256)), 'mp4', 'album.mp4', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860011, 'parity-860011', 'parity-u-860011', 'document', 10000, 1,
	 UNHEX(SHA2('parity 860011', 256)), 'pdf', 'notes.pdf', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860012, 'parity-860012', 'parity-u-860012', 'photo', 50000, 1,
	 UNHEX(SHA2('parity 860012', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860013, 'parity-860013', 'parity-u-860013', 'video_note', 300000, 1,
	 UNHEX(SHA2('parity 860013', 256)), 'mp4', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(860014, 'parity-860014', 'parity-u-860014', 'photo', 30000, 1,
	 UNHEX(SHA2('parity 860014', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00');

INSERT INTO telegram_users (id, first_name, last_name, type, msg_count, created_at, updated_at)
	VALUES (9299000030, 'Tono', 'Wijaya', 'regular', 2, '2025-04-04 04:04:00',
	 '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES
	(-1001000000030, 'supergroup', 'Media Lab', 'Every kind of media.', 19,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00'),
	(-1001000000031, 'channel', 'Channel News', 'A channel.', 1,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title) VALUES
	(-1001000000030, 9299000001, 'administrator', ''),
	(-1001000000030, 9299000007, 'administrator', 'mod');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 19)
SELECT 3000 + n, -1001000000030, (600000 + n) << 20,
	ELT(1 + n % 4, 9299000001, 9299000002, 9299000003, 9299000004),
	1794931200 - 7200 + 60 * n, 'text', CONCAT('media message ', n)
FROM seq;

UPDATE telegram_group_messages SET content_type = 'video', text = 'a clip', file_id = 860001,
	tdlib_type = 'messageVideo', content_extra = JSON_OBJECT('width', 1280, 'height', 720,
	'duration', 42, 'mime', 'video/mp4', 'minithumbnail',
	JSON_OBJECT('width', 40, 'height', 22, 'data', '/9j/4AAQ')) WHERE id = 3001;
UPDATE telegram_group_messages SET content_type = 'video', text = NULL, file_id = 860002
	WHERE id = 3002;
UPDATE telegram_group_messages SET content_type = 'animation', text = NULL, file_id = 860003,
	tdlib_type = 'messageAnimation',
	content_extra = JSON_OBJECT('width', 320, 'height', 240, 'duration', 3, 'minithumbnail',
	JSON_OBJECT('width', 40, 'height', 30, 'data', 'AAEC')) WHERE id = 3003;
INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(3001, 'cover', 0, 860014);
UPDATE telegram_group_messages SET content_type = 'sticker', text = NULL, file_id = 860004,
	tdlib_type = 'messageSticker', content_extra = JSON_OBJECT('emoji',
	CONVERT(X'E29DA4' USING utf8mb4), 'format', 'webp', 'width', 512, 'height', 512)
	WHERE id = 3004;
UPDATE telegram_group_messages SET content_type = 'sticker', text = NULL, file_id = 860005
	WHERE id = 3005;
UPDATE telegram_group_messages SET content_type = 'sticker', text = NULL, file_id = 860006,
	tdlib_type = 'messageSticker', content_extra = JSON_OBJECT('emoji', 'ok', 'format', 'tgs')
	WHERE id = 3006;
UPDATE telegram_group_messages SET content_type = 'voice', text = NULL, file_id = 860007,
	tdlib_type = 'messageVoiceNote', content_extra = JSON_OBJECT('duration', 7,
	'mime', 'audio/ogg', 'waveform', 'QQxSzPk=') WHERE id = 3007;
UPDATE telegram_group_messages SET content_type = 'audio', text = NULL, file_id = 860008,
	tdlib_type = 'messageAudio', content_extra = JSON_OBJECT('duration', 200,
	'title', 'Song', 'performer', 'Band') WHERE id = 3008;

UPDATE telegram_group_messages SET content_type = 'photo', text = NULL, file_id = 860009,
	media_album_id = 8001, content_extra = JSON_OBJECT('width', 800, 'height', 600)
	WHERE id = 3009;
UPDATE telegram_group_messages SET content_type = 'video', text = NULL, file_id = 860010,
	media_album_id = 8001, deleted_at = '2026-11-17 12:00:00' WHERE id = 3010;
UPDATE telegram_group_messages SET content_type = 'document', text = 'mixed album',
	file_id = 860011, media_album_id = 8001 WHERE id = 3011;
UPDATE telegram_group_messages SET sender_user_id = 9299000002 WHERE id IN (3009, 3010, 3011);

UPDATE telegram_group_messages SET content_type = 'photo', text = 'now a photo',
	file_id = 860012, edit_date = 1794931200 - 7200 + 60 * 12 + 50 WHERE id = 3012;
INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, file_id,
	edit_date, content_extra) VALUES
	(3012, 'text', 'before the photo', NULL, 0, NULL),
	(3012, 'video', 'a video first', 860010, 1794931200 - 7200 + 60 * 12 + 20,
	 JSON_OBJECT('duration', 5));

UPDATE telegram_group_messages SET is_forwarded = 1 WHERE id IN (3013, 3014);
INSERT INTO telegram_group_message_fwd_info (group_message_id, origin_type,
	origin_sender_user_id, origin_sender_name, origin_chat_id, origin_message_id, origin_date)
	VALUES
	(3013, 'user', 9299000003, NULL, NULL, NULL, 1794900000),
	(3014, 'channel', NULL, NULL, -1001000000031, 77 << 20, 1794800000);

UPDATE telegram_group_messages SET sender_user_id = NULL, sender_chat_id = -1001000000031,
	author_signature = 'Editor' WHERE id = 3015;

UPDATE telegram_group_messages SET reply_to_chat_id = -1001000000030,
	reply_to_msg_id = (600000 + 10) << 20, reply_to_id = 3010 WHERE id = 3016;
UPDATE telegram_group_messages SET reply_to_chat_id = -1001000000030,
	reply_to_msg_id = 5 << 20 WHERE id = 3017;

UPDATE telegram_group_messages SET content_type = 'service', text = NULL,
	service_type = 'chat_add_members', sender_user_id = 9299000001,
	content_extra = JSON_OBJECT('user_ids', JSON_ARRAY(9299000004)) WHERE id = 3018;

UPDATE telegram_group_messages SET content_type = 'video_note', text = NULL, file_id = 860013,
	tdlib_type = 'messageVideoNote', content_extra = JSON_OBJECT('duration', 9, 'length', 240,
	'waveform', 'QQxSzPk=') WHERE id = 3019;

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_chat_id, is_channel_post,
	author_signature, date, content_type, text) VALUES
	(3101, -1001000000031, 77 << 20, -1001000000031, 1, 'Ana', 1794800000, 'text', 'news');

INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing, date,
	content_type, text, file_id, content_extra) VALUES
	(3001, 9299000030, 910001 << 20, 9299000030, 0, 1794931200 - 1800, 'photo', 'look',
	 860009, JSON_OBJECT('width', 800, 'height', 600)),
	(3002, 9299000030, 910002 << 20, NULL, 1, 1794931200 - 1700, 'voice', NULL,
	 860007, NULL);
