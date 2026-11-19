-- Dummy link previews with their type's extras and their own media,
-- for the link preview card's tests; loaded after "sample" and "chats"
-- (their groups and messages are referred to). Invented; nothing here
-- comes from a real archive.
--
-- Ids, beside the others':
--   files      871001 - 871006
--   messages   group rows 2101 - 2109 (Open Lab, exposed), message id
--              (810000 + n) << 20 for row 2100 + n
--   usernames  openlabtest (Open Lab), paginglab (Paging Lab, not
--              exposed), oldlab (released)
-- The previews, by row:
--   2101  video: stored medium 871002, stored photo 871001, a
--         minithumbnail in extra
--   2102  embedded video player: medium 871004 not stored, photo 871003
--   2103  embedded audio player without a medium or photo
--   2104  audio: medium 871005, photo 871006
--   2105  message, t.me/c/ link to Paging Lab's message 5
--   2106  message, a username link (mixed case) to Open Lab's 800001
--   2107  message, a username link to an Open Lab message never logged
--   2108  message, a link through a released username
--   2109  article with its photo's size and a minithumbnail

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(871001, 'lpm-fixture-871001', 'lpm-u-871001', 'photo', 1000, 1,
	 UNHEX(SHA2('lpm photo 871001', 256)), 'jpg', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(871002, 'lpm-fixture-871002', 'lpm-u-871002', 'video', 50000, 1,
	 UNHEX(SHA2('lpm video 871002', 256)), 'mp4', 'clip.mp4', '2026-11-19 10:00:00',
	 '2026-11-19 10:00:00'),
	(871003, 'lpm-fixture-871003', 'lpm-u-871003', 'photo', 1000, 1,
	 UNHEX(SHA2('lpm photo 871003', 256)), 'jpg', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(871004, 'lpm-fixture-871004', 'lpm-u-871004', 'video', 90000, 0,
	 NULL, 'mp4', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(871005, 'lpm-fixture-871005', 'lpm-u-871005', 'audio', 40000, 1,
	 UNHEX(SHA2('lpm audio 871005', 256)), 'mp3', 'song.mp3', '2026-11-19 10:00:00',
	 '2026-11-19 10:00:00'),
	(871006, 'lpm-fixture-871006', 'lpm-u-871006', 'photo', 1000, 1,
	 UNHEX(SHA2('lpm photo 871006', 256)), 'jpg', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00');

INSERT INTO telegram_group_usernames (group_id, username, kind, position) VALUES
	(-1001000000021, 'openlabtest', 'active', 0),
	(-1001000000020, 'paginglab', 'active', 0),
	(NULL, 'oldlab', 'active', 0);

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 9)
SELECT 2100 + n, -1001000000021, (810000 + n) << 20, 9299000001,
	1794931200 - 250 + 10 * n, 'text', CONCAT('a link ', n)
FROM seq;

INSERT INTO telegram_link_previews (group_message_id, url, display_url, site_name, title,
	description, author, type, photo_file_id, extra) VALUES
	(2101, 'https://video.example.org/clip', 'video.example.org/clip', 'Example Video',
	 'A clip', 'A short clip.', '', 'video', 871001,
	 JSON_OBJECT('width', 1280, 'height', 720, 'duration', 42, 'mime', 'video/mp4',
		'streaming', TRUE, 'file_name', 'clip.mp4',
		'minithumbnail', JSON_OBJECT('width', 40, 'height', 22, 'data', 'AAAA'))),
	(2102, 'https://tube.example.org/watch?v=x', 'tube.example.org/watch?v=x', 'ExampleTube',
	 'A talk', 'A long talk.', 'Some Channel', 'embedded_video_player', 871003,
	 JSON_OBJECT('embed_url', 'https://tube.example.org/embed/x', 'embed_width', 1280,
		'embed_height', 720, 'duration', 213,
		'media', JSON_OBJECT('width', 640, 'height', 360, 'duration', 30, 'mime', 'video/mp4',
			'minithumbnail', JSON_OBJECT('width', 40, 'height', 22, 'data', 'AAAA')))),
	(2103, 'https://sound.example.org/t', 'sound.example.org/t', 'ExampleSound', 'A track',
	 '', '', 'embedded_audio_player', NULL,
	 JSON_OBJECT('embed_url', 'https://sound.example.org/player/t', 'duration', 180)),
	(2104, 'https://music.example.org/song.mp3', 'music.example.org/song.mp3', 'Example Music',
	 'song.mp3', '', '', 'audio', 871006,
	 JSON_OBJECT('duration', 200, 'mime', 'audio/mpeg', 'audio_title', 'Song',
		'performer', 'Band', 'file_name', 'song.mp3')),
	(2105, 'https://t.me/c/1000000020/734007', 't.me/c/1000000020/734007', 'Telegram',
	 'Paging Lab', 'paging message 5', '', 'message', NULL,
	 JSON_OBJECT('chat_id', -1001000000020, 'message_id', (700000 + 5) << 20)),
	(2106, 'https://t.me/OpenLabTest/1', 't.me/OpenLabTest/1', 'Telegram', 'Open Lab',
	 'open hello', '', 'message', NULL,
	 JSON_OBJECT('username', 'OpenLabTest', 'message_id', 800001 << 20)),
	(2107, 'https://t.me/openlabtest/2', 't.me/openlabtest/2', 'Telegram', 'Open Lab', '',
	 '', 'message', NULL, JSON_OBJECT('username', 'openlabtest', 'message_id', 999999)),
	(2108, 'https://t.me/oldlab/3', 't.me/oldlab/3', 'Telegram', 'Old Lab', '', '',
	 'message', NULL, JSON_OBJECT('username', 'oldlab', 'message_id', 800001 << 20)),
	(2109, 'https://example.org/page', 'example.org/page', 'Example', 'A page', 'Words.', '',
	 'article', 871001,
	 JSON_OBJECT('width', 800, 'height', 418,
		'minithumbnail', JSON_OBJECT('width', 40, 'height', 21, 'data', 'AAAA')));

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(2101, 'preview_media', 0, 871002),
	(2102, 'preview_media', 0, 871004),
	(2104, 'preview_media', 0, 871005);
