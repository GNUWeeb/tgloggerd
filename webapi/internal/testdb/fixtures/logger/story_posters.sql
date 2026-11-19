-- Stories with posters that have a public username, for the story
-- detail's username, loaded after "sample". Invented, like "sample".
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000041 (Story Lab, not exposed)
--   messages   group rows 4101 - 4102 (message ids 710001 - 710002)
-- Andi sends both: 1 shares a story of Dewi (dewikusuma), 2 one of
-- Linux Kernel Indonesia (linuxkernelid). A member granted Story Lab
-- sees Andi but neither poster.

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000041, 'supergroup', 'Story Lab', 'Shared stories.', 2,
	 '2025-07-07 07:07:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra) VALUES
	(4101, -1001000000041, 710001, 9299000004, 1794927660, 'story', NULL, 'messageStory',
	 '{"poster_chat_id":9299000001,"story_id":31}'),
	(4102, -1001000000041, 710002, 9299000004, 1794927720, 'story', NULL, 'messageStory',
	 '{"poster_chat_id":-1001000000002,"story_id":32}');
