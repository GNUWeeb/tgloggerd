-- Custom emoji and message effects (docs/design/message-types.md
-- sections 4.11 and 4.12), for CustomEmoji and MessageEffect. Loaded
-- after "sample", whose Dewi sends the messages. Invented; nothing here
-- comes from a real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000080 (Emoji Lab, a forum supergroup, not
--              exposed)
--   files      876001 - 876010 (876006 - 876010 for the tests' fake
--              daemon to record)
--   messages   group rows 8001 - 8006 (Emoji Lab, n = 1 - 6), message
--              ids 730000 + n
-- Custom emoji, by the last digit of 500000000000000000n:
--   1  stored, webp, its file stored (876001)
--   2  stored, tgs, drawn in the text colour, its file not stored
--      (876002)
--   3  not stored; a fake daemon knows it
--   4  not stored; Telegram does not know it
--   5  stored, its format unknown, its file stored (876003)
-- Message effects, by the last digit of 510000000000000000n:
--   1  stored, with a static icon (876004)
--   2  stored, an emoji reaction without a static icon, its select
--      animation stored (876005)
--   3  not stored; a fake daemon knows it
-- Emoji Lab's messages, by n:
--   1  text with custom emoji 1, 2, 3, 4 and 5 (topic 21)
--   2  text with reactions: custom emoji 1 and 4, a thumbs up; effect 1
--   3  text with effect 2               4  text with effect 3
--   5  forum_topic_created of topic 21 with custom emoji 1 as icon
--   6  forum_topic_edited of topic 21, its custom icon removed
-- Topics: 21 "Stickers" (icon: custom emoji 5 now), 22 "Plain" (none).

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(876001, 'emo-876001', 'emo-u-876001', 'sticker', 2000, 1,
	 UNHEX(SHA2('emo 876001', 256)), 'webp', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(876002, 'emo-876002', 'emo-u-876002', 'sticker', 3000, 0,
	 NULL, 'tgs', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(876003, 'emo-876003', 'emo-u-876003', 'sticker', 2100, 1,
	 UNHEX(SHA2('emo 876003', 256)), 'webp', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(876004, 'emo-876004', 'emo-u-876004', 'sticker', 1500, 1,
	 UNHEX(SHA2('emo 876004', 256)), 'webp', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00'),
	(876005, 'emo-876005', 'emo-u-876005', 'sticker', 4000, 1,
	 UNHEX(SHA2('emo 876005', 256)), 'tgs', '', '2026-11-19 10:00:00', '2026-11-19 10:00:00');

INSERT INTO telegram_custom_emoji (custom_emoji_id, emoji, set_id, format, needs_repainting,
	file_id) VALUES
	(5000000000000000001, _utf8mb4 0xF09F9982, 77, 'webp', 0, 876001),
	(5000000000000000002, _utf8mb4 0xF09F9880, 77, 'tgs', 1, 876002),
	(5000000000000000005, _utf8mb4 0xF09FA496, NULL, NULL, 0, 876003);

INSERT INTO telegram_message_effects (effect_id, emoji, is_premium, effect_type,
	static_icon_file_id, select_animation_file_id, effect_animation_file_id,
	sticker_file_id, sticker_format) VALUES
	(5100000000000000001, _utf8mb4 0xF09F94A5, 0, 'emoji_reaction', 876004, 876005, NULL, NULL, NULL),
	(5100000000000000002, _utf8mb4 0xF09F8E89, 0, 'emoji_reaction', NULL, 876005, NULL, NULL, NULL);

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000080, 'supergroup', 'Emoji Lab', 'Custom emoji.', 6,
	 '2025-06-06 06:06:00', '2026-11-19 10:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, entities, tdlib_type, content_extra, service_type, meta,
	reactions) VALUES
	(8001, -1001000000080, 730001, 9299000001, 1794931260, 'text', 'aa bb cc dd ee',
	 '[{"offset":0,"length":2,"type":"custom_emoji","custom_emoji_id":5000000000000000001},{"offset":3,"length":2,"type":"custom_emoji","custom_emoji_id":5000000000000000002},{"offset":6,"length":2,"type":"custom_emoji","custom_emoji_id":"5000000000000000003"},{"offset":9,"length":2,"type":"custom_emoji","custom_emoji_id":5000000000000000004},{"offset":12,"length":2,"type":"custom_emoji","custom_emoji_id":5000000000000000005},{"offset":0,"length":2,"type":"bold"}]',
	 'messageText', NULL, NULL, NULL, NULL),
	(8002, -1001000000080, 730002, 9299000001, 1794931320, 'text', 'Reacted', NULL,
	 'messageText', NULL, NULL, '{"effect_id":"5100000000000000001"}',
	 '[{"custom_emoji_id":"5000000000000000001","count":3},{"custom_emoji_id":"5000000000000000004","count":1},{"emoji":"\\ud83d\\udc4d","count":2}]'),
	(8003, -1001000000080, 730003, 9299000001, 1794931380, 'text', 'Party', NULL,
	 'messageText', NULL, NULL, '{"effect_id":"5100000000000000002"}', NULL),
	(8004, -1001000000080, 730004, 9299000001, 1794931440, 'text', 'Fresh', NULL,
	 'messageText', NULL, NULL, '{"effect_id":"5100000000000000003"}', NULL),
	(8005, -1001000000080, 730005, 9299000001, 1794931500, 'service', 'created the topic', NULL,
	 'messageForumTopicCreated', '{"name":"Stickers","icon_color":7322096,"icon_custom_emoji_id":"5000000000000000001"}',
	 'forum_topic_created', NULL, NULL),
	(8006, -1001000000080, 730006, 9299000001, 1794931560, 'service', 'edited the topic', NULL,
	 'messageForumTopicEdited', '{"name":"Stickers","icon_edited":true}',
	 'forum_topic_edited', NULL, NULL);

INSERT INTO telegram_forum_topics (chat_id, topic_id, name, icon_color, icon_custom_emoji_id,
	creation_date, creator_user_id, is_general, is_closed, is_hidden, is_pinned) VALUES
	(-1001000000080, 21, 'Stickers', 7322096, 5000000000000000005, 1794931500, 9299000001,
	 0, 0, 0, 1),
	(-1001000000080, 22, 'Plain', 16749490, NULL, 1794931000, 9299000001, 0, 0, 0, 0);
UPDATE telegram_group_messages SET topic_kind = 'forum', topic_id = 21
	WHERE id IN (8001, 8005, 8006);
