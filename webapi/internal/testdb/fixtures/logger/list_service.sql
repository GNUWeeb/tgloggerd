-- Service messages for the message lists' service_type, loaded after
-- "sample": a service row and a plain row, the newest of their list,
-- in a group and in a private chat. Invented, like "sample".
--
-- Ids, beside sample's:
--   messages   group row ids 600 - 601 (Telegram message ids 431001 <<
--              20 on), private row ids 600 - 601 (621001 << 20 on)

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, service_type) VALUES
	(600, -1001000000003, 431001 << 20, 9299000004, 1794931200 - 60, 'service',
	 'pinned a message', 'messagePinMessage', 'pin_message'),
	(601, -1001000000003, 431002 << 20, 9299000004, 1794931200 - 50, 'text',
	 'pinned the benchmark', NULL, NULL);

UPDATE telegram_groups SET msg_count = msg_count + 2 WHERE id = -1001000000003;

INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing,
	date, content_type, text, tdlib_type, service_type) VALUES
	(600, 9299000002, 621001 << 20, 9299000002, 0, 1794931200 - 60, 'service', NULL,
	 'messageContactRegistered', 'contact_registered'),
	(601, 9299000002, 621002 << 20, 9299000002, 0, 1794931200 - 50, 'text',
	 'welcome aboard', NULL, NULL);
