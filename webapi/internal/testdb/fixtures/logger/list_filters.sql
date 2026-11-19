-- Message kinds for the message lists' filters, loaded after "sample":
-- a members-added service row, a poll, a location and a photo in the
-- group of sample's messages 9 - 11. Invented, like "sample".
--
-- Ids, beside sample's:
--   messages   group row ids 700 - 703 (Telegram message ids 421001 <<
--              20 on)

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, service_type) VALUES
	(700, -1001000000002, 421001 << 20, 9299000007, 1794931200 - 8200, 'service',
	 'added a member to the group', 'messageChatAddMembers', 'chat_add_members'),
	(701, -1001000000002, 421002 << 20, 9299000007, 1794931200 - 8100, 'poll',
	 'which compiler?', 'messagePoll', NULL),
	(702, -1001000000002, 421003 << 20, 9299000001, 1794931200 - 8000, 'location',
	 NULL, 'messageLocation', NULL),
	(703, -1001000000002, 421004 << 20, 9299000001, 1794931200 - 7900, 'photo',
	 'the board', 'messagePhoto', NULL);

UPDATE telegram_groups SET msg_count = msg_count + 4 WHERE id = -1001000000002;
