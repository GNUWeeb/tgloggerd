-- More dummy data for the search tests, loaded after "sample": a user
-- whose name needs HTML escaping, a forwarded message in the group
-- that had none, and the forward origins of the forwarded messages.
-- Invented, like "sample".
--
-- Ids, beside sample's:
--   users      9299100001
--   messages   group row id 100 (Telegram message id 440001 << 20)

INSERT INTO telegram_users (id, first_name, last_name, type, msg_count, created_at,
	updated_at) VALUES
	(9299100001, '<i>Tom & "Jerry"</i>', 'O''Neil', 'regular', 1,
	 '2026-11-01 08:00:00', '2026-11-17 15:50:00');

INSERT INTO telegram_user_usernames (user_id, username, kind, position) VALUES
	(9299100001, 'tom_jerry', 'active', 0);

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, is_forwarded) VALUES
	(100, -1001000000004, 440001 << 20, 9299100001, 1794931200 - 600, 'text',
	 'templates & <generics> in C++', 1);

UPDATE telegram_groups SET msg_count = msg_count + 1 WHERE id = -1001000000004;

-- Message 11 of sample is forwarded from Bima; message 100 from a user
-- who hides their account.
INSERT INTO telegram_group_message_fwd_info (group_message_id, origin_type,
	origin_sender_user_id, origin_sender_name, origin_date) VALUES
	(11, 'user', 9299000002, NULL, 1794931200 - 90000),
	(100, 'hidden_user', NULL, 'Secret Admirer', 1794931200 - 4000);
