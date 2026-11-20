-- Dummy link previews of the types named since "unsupported" and the
-- media kept since for the older ones (docs/design/message-types.md
-- sections 4.8 and 4.3), for the link preview card's tests; loaded after
-- "sample" and "chats" (their users, groups and messages are referred
-- to). Invented; nothing here comes from a real archive.
--
-- Ids, beside the others':
--   files         872001 - 872021
--   custom emoji  5000000000000000091 (stored, webp, file 872019)
--   messages      group rows 2111 - 2127 (Open Lab, exposed), message id
--                 (811000 + n) << 20 for row 2110 + n
-- The previews, by row:
--   2111  video: medium 872002, photo 872001, its cover 872003 (pos 1)
--   2112  document: its file 872004, photo 872001 (the thumbnail)
--   2113  sticker: its sticker 872005 (webp)
--   2114  video_note: medium 872007, photo 872006
--   2115  voice_note: medium 872008 with a waveform
--   2116  album: a photo (872009), a video (872010), an empty item
--   2117  background: a pattern (872011, png), photo 872012
--   2118  sticker_set: stickers 872013 (tgs), none, 872014 (webm)
--   2119  gift_collection: one icon, 872015
--   2120  gift_auction: a gift sticker 872016
--   2121  upgraded_gift: model 872017, symbol 872018, owned by Dewi
--   2122  live_story by Dewi
--   2123  theme
--   2124  text_composition_style with the stored custom emoji
--   2125  unsupported (as rows logged before the new names), photo
--         872001
--   2126  story_album: photo icon 872020, video icon 872021
--   2127  direct_messages_chat with its photo 872001

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at)
SELECT 872000 + n, CONCAT('lpt-fixture-', 872000 + n), CONCAT('lpt-u-', 872000 + n),
	ELT(n, 'photo', 'video', 'photo', 'document', 'sticker', 'photo', 'video_note', 'voice',
		'photo', 'video', 'document', 'photo', 'sticker', 'sticker', 'sticker', 'sticker',
		'sticker', 'sticker', 'sticker', 'photo', 'video'),
	1000 + n, 1, UNHEX(SHA2(CONCAT('lpt file ', n), 256)),
	ELT(n, 'jpg', 'mp4', 'jpg', 'pdf', 'webp', 'jpg', 'mp4', 'ogg', 'jpg', 'mp4', 'png', 'jpg',
		'tgs', 'webm', 'tgs', 'tgs', 'tgs', 'tgs', 'webp', 'jpg', 'mp4'),
	IF(n = 4, 'paper.pdf', ''), '2026-11-20 10:00:00', '2026-11-20 10:00:00'
FROM (WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 21)
	SELECT n FROM seq) s;

INSERT INTO telegram_custom_emoji (custom_emoji_id, emoji, set_id, format, needs_repainting,
	file_id) VALUES
	(5000000000000000091, _utf8mb4 0xE29CA8, 91, 'webp', 0, 872019);

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 17)
SELECT 2110 + n, -1001000000021, (811000 + n) << 20, 9299000001,
	1794931200 - 200 + 10 * n, 'text', CONCAT('a typed link ', n)
FROM seq;

INSERT INTO telegram_link_previews (group_message_id, url, display_url, site_name, title,
	description, author, type, photo_file_id, extra) VALUES
	(2111, 'https://t.me/lab/1', 't.me/lab/1', 'Telegram', 'A clip', '', '', 'video', 872001,
	 JSON_OBJECT('width', 1280, 'height', 720, 'duration', 12, 'mime', 'video/mp4',
		'cover', TRUE)),
	(2112, 'https://t.me/lab/2', 't.me/lab/2', 'Telegram', 'A paper', '', '', 'document',
	 872001, JSON_OBJECT('mime', 'application/pdf', 'file_name', 'paper.pdf')),
	(2113, 'https://t.me/lab/3', 't.me/lab/3', 'Telegram', 'A sticker', '', '', 'sticker',
	 NULL, JSON_OBJECT('emoji', 'x', 'sticker_id', '31', 'set_id', '91', 'format', 'webp',
		'width', 512, 'height', 512)),
	(2114, 'https://t.me/lab/4', 't.me/lab/4', 'Telegram', 'A round', '', '', 'video_note',
	 872006, JSON_OBJECT('duration', 9, 'length', 384,
		'minithumbnail', JSON_OBJECT('width', 40, 'height', 40, 'data', 'AAAA'))),
	(2115, 'https://t.me/lab/5', 't.me/lab/5', 'Telegram', 'A voice', '', '', 'voice_note',
	 NULL, JSON_OBJECT('duration', 5, 'mime', 'audio/ogg', 'waveform', 'AAAAAAAA',
		'transcript', 'hello')),
	(2116, 'https://t.me/lab/6', 't.me/lab/6', 'Telegram', 'An album', '', '', 'album', NULL,
	 JSON_OBJECT('caption', 'Two of them',
		'items', JSON_ARRAY(JSON_OBJECT('kind', 'photo', 'width', 800, 'height', 600),
			JSON_OBJECT('kind', 'video', 'width', 640, 'height', 360, 'duration', 3,
				'file_name', 'a.mp4'),
			NULL))),
	(2117, 'https://t.me/bg/pattern', 't.me/bg/pattern', 'Telegram', 'A background', '', '',
	 'background', 872012,
	 JSON_OBJECT('background_type', 'pattern', 'intensity', 50, 'is_moving', TRUE,
		'fill', JSON_OBJECT('type', 'gradient', 'colors', JSON_ARRAY(255, 4278190335),
			'rotation_angle', 45),
		'mime', 'image/png', 'file_name', 'pattern.png')),
	(2118, 'https://t.me/addstickers/lab', 't.me/addstickers/lab', 'Telegram', 'A set', '', '',
	 'sticker_set', NULL,
	 JSON_OBJECT('set_id', '91', 'stickers', JSON_ARRAY(
		JSON_OBJECT('emoji', 'a', 'sticker_id', '1', 'format', 'tgs', 'width', 512, 'height', 512),
		JSON_OBJECT('emoji', 'b', 'sticker_id', '2', 'format', 'tgs'),
		JSON_OBJECT('emoji', 'c', 'sticker_id', '3', 'format', 'webm', 'width', 512,
			'height', 512)))),
	(2119, 'https://t.me/lab/gifts', 't.me/lab/gifts', 'Telegram', 'Gifts', '', '',
	 'gift_collection', NULL,
	 JSON_OBJECT('icons', JSON_ARRAY(JSON_OBJECT('emoji', 'g', 'sticker_id', '4',
		'format', 'tgs')))),
	(2120, 'https://t.me/auction/lab', 't.me/auction/lab', 'Telegram', 'An auction', '', '',
	 'gift_auction', NULL,
	 JSON_OBJECT('gift', JSON_OBJECT('id', '6001', 'star_count', 500, 'emoji', 'h'),
		'auction_start_date', 1794000000, 'auction_end_date', 1795000000,
		'gifts_per_round', 10)),
	(2121, 'https://t.me/nft/LabGift-7', 't.me/nft/LabGift-7', 'Telegram', 'Lab Gift #7', '',
	 '', 'upgraded_gift', NULL,
	 JSON_OBJECT('gift', JSON_OBJECT('id', '7001', 'regular_gift_id', '6001',
		'title', 'Lab Gift', 'name', 'LabGift-7', 'number', 7,
		'owner', JSON_OBJECT('user_id', 9299000001),
		'model', JSON_OBJECT('name', 'Round', 'rarity_per_mille', 20),
		'backdrop', JSON_OBJECT('name', 'Night', 'center_color', 1, 'edge_color', 2,
			'symbol_color', 3, 'text_color', 4, 'rarity', 'rare')))),
	(2122, 'https://t.me/dewikusuma/s/live', 't.me/dewikusuma/s/live', 'Telegram', 'Live', '',
	 '', 'live_story', NULL, JSON_OBJECT('poster_chat_id', 9299000001, 'story_id', 7)),
	(2123, 'https://t.me/addtheme/lab', 't.me/addtheme/lab', 'Telegram', 'A theme', '', '',
	 'theme', NULL,
	 JSON_OBJECT('base_theme', 'night', 'accent_color', 0,
		'background', JSON_OBJECT('name', 'labbg', 'dark', TRUE, 'type', 'fill',
			'fill', JSON_OBJECT('type', 'solid', 'colors', JSON_ARRAY(123),
				'rotation_angle', 0)),
		'outgoing_message_fill', JSON_OBJECT('type', 'gradient',
			'colors', JSON_ARRAY(1, 2), 'rotation_angle', 90),
		'outgoing_message_accent_color', 16711935)),
	(2124, 'https://t.me/addstyle/lab', 't.me/addstyle/lab', 'Telegram', 'A style', '', '',
	 'text_composition_style', NULL, JSON_OBJECT('custom_emoji_id', '5000000000000000091')),
	(2125, 'https://t.me/lab/old', 't.me/lab/old', 'Telegram', 'Old', '', '', 'unsupported',
	 872001, NULL),
	(2126, 'https://t.me/dewikusuma/a/1', 't.me/dewikusuma/a/1', 'Telegram', 'Stories', '', '',
	 'story_album', 872020,
	 JSON_OBJECT('width', 320, 'height', 320,
		'video', JSON_OBJECT('width', 240, 'height', 240, 'duration', 2, 'mime', 'video/mp4'))),
	(2127, 'https://t.me/lab?direct', 't.me/lab?direct', 'Telegram', 'Messages', '', '',
	 'direct_messages_chat', 872001, NULL);

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(2111, 'preview_media', 0, 872002),
	(2111, 'preview_media', 1, 872003),
	(2112, 'preview_media', 0, 872004),
	(2113, 'preview_media', 0, 872005),
	(2114, 'preview_media', 0, 872007),
	(2115, 'preview_media', 0, 872008),
	(2116, 'preview_media', 0, 872009),
	(2116, 'preview_media', 1, 872010),
	(2117, 'preview_media', 0, 872011),
	(2118, 'preview_media', 0, 872013),
	(2118, 'preview_media', 2, 872014),
	(2119, 'preview_media', 0, 872015),
	(2120, 'preview_media', 0, 872016),
	(2121, 'preview_media', 0, 872017),
	(2121, 'preview_media', 1, 872018),
	(2126, 'preview_media', 0, 872021);
