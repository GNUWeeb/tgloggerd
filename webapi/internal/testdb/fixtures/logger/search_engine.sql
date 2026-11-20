-- More dummy data for the search engine's tests (internal/searchengine),
-- loaded after "sample": messages for phrase, proximity, fuzzy and
-- prefix searches, spread over groups a restricted viewer may not see.
-- Invented, like "sample".
--
-- Ids, beside sample's:
--   messages   group row ids 300 - 306 (Telegram message ids 470001 <<
--              20 on), private row id 300 (670001 << 20)

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text) VALUES
	(300, -1001000000001, 470001 << 20, 9299000003, 1794931200 - 500, 'text',
	 'the quick brown fox jumps over the lazy dog'),
	(301, -1001000000002, 470002 << 20, 9299000001, 1794931200 - 490, 'text',
	 'a quick fox in the kernel group'),
	(302, -1001000000001, 470003 << 20, 9299000001, 1794931200 - 480, 'text',
	 'quick thinking saved the brown fox'),
	(303, -1001000000002, 470004 << 20, 9299000007, 1794931200 - 470, 'text',
	 'kernel panic on boot after the update'),
	(304, -1001000000001, 470005 << 20, 9299000002, 1794931200 - 460, 'text',
	 'another kernel panic, same trace'),
	(305, -1001000000001, 470006 << 20, 9299000002, 1794931200 - 450, 'text',
	 'the kernal oops is back'),
	(306, -1001000000003, 470007 << 20, 9299000004, 1794931200 - 440, 'text',
	 'quick question about lifetimes');

UPDATE telegram_groups SET msg_count = msg_count + 4 WHERE id = -1001000000001;
UPDATE telegram_groups SET msg_count = msg_count + 2 WHERE id = -1001000000002;
UPDATE telegram_groups SET msg_count = msg_count + 1 WHERE id = -1001000000003;

INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing, date,
	content_type, text) VALUES
	(300, 9299000001, 670001 << 20, 9299000001, 0, 1794931200 - 300, 'text',
	 'a quick fox story for you');

UPDATE telegram_users SET msg_count = msg_count + 3 WHERE id = 9299000001;
UPDATE telegram_users SET msg_count = msg_count + 2 WHERE id = 9299000002;
UPDATE telegram_users SET msg_count = msg_count + 1
	WHERE id IN (9299000003, 9299000004, 9299000007);
