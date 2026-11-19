-- Dummy messages with Telegram text entities (formatting), for the
-- TextEntity fields of the chat, the message page, edits, polls and
-- checklists. Loaded after "sample", whose users are the senders and the
-- people mentioned. Invented; nothing here comes from a real archive.
-- The entities are as the daemon stores them (format_entities_json in
-- src/tgloggerd/MessageExtract.cpp), offsets in UTF-16 code units.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000060 (Format Lab, supergroup, not exposed)
--   polls      5550101 (a quiz with formatted question and explanation)
--   messages   group rows 6001 - 6008 (Format Lab, n = 1 - 8), private
--              row 6101 (with Dewi)
-- Format Lab's message n has message id 760000 + n. Its senders are Dewi
-- (odd n) and Andi (even n), and Dewi is its admin, so a member granted
-- Format Lab alone sees those two and not Bima or Sari. By n:
--   1  every entity kind: a custom emoji (an id past 2^53) before the
--      rest, so UTF-16 and code point offsets differ; @dewikusuma
--      (visible to the member), @bima_pratama (not), @SariW (Sari's
--      active username in other case; not visible to the member),
--      @sari_gw (disabled), @old_handle (released), @nobody_here
--      (unknown), a url, a text_url, mention_names of Bima and Dewi, a
--      hashtag, bold, a spoiler, an email, a type the web does not
--      know, and a pre_code block in go
--   2  an edited message: bold now, a mention of @dewikusuma before
--   3  poll 5550101: an italic question, an explanation naming Bima
--   4  checklist: a hashtag task, a task naming Dewi, a plain task
--   5  service pin_message with (unexpected) entities: none are sent
--   6  entities that are not an array: none are sent
--   7  entities with one element lacking its type: none are sent
--   8  plain text

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000060, 'supergroup', 'Format Lab', 'Formatted messages.', 8,
	 '2025-07-07 07:07:00', '2026-11-18 15:00:00');

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title) VALUES
	(-1001000000060, 9299000001, 'administrator', '');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, entities, tdlib_type, content_extra, service_type, edit_date) VALUES
	(6001, -1001000000060, 760001, 9299000001, 1794927660, 'text', '😀 Hi @dewikusuma, @bima_pratama, @SariW, @sari_gw, @old_handle and @nobody_here. See https://example.org and the docs, ask Bima or Dewi. #release bold secret a@example.org odd
fmt.Println()', '[{"offset":0,"length":2,"type":"custom_emoji","custom_emoji_id":5368324170671202286},{"offset":6,"length":11,"type":"mention"},{"offset":19,"length":13,"type":"mention"},{"offset":34,"length":6,"type":"mention"},{"offset":42,"length":8,"type":"mention"},{"offset":52,"length":11,"type":"mention"},{"offset":68,"length":12,"type":"mention"},{"offset":86,"length":19,"type":"url"},{"offset":114,"length":4,"type":"text_url","url":"https://example.org/docs"},{"offset":124,"length":4,"type":"mention_name","user_id":9299000002},{"offset":132,"length":4,"type":"mention_name","user_id":9299000001},{"offset":138,"length":8,"type":"hashtag"},{"offset":147,"length":4,"type":"bold"},{"offset":152,"length":6,"type":"spoiler"},{"offset":159,"length":13,"type":"email"},{"offset":173,"length":3,"type":"future_thing"},{"offset":177,"length":13,"type":"pre_code","language":"go"}]', 'messageText', NULL, NULL, 0),
	(6002, -1001000000060, 760002, 9299000004, 1794927720, 'text', 'Fixed now', '[{"offset":6,"length":3,"type":"bold"}]', 'messageText', NULL, NULL, 1794927750),
	(6003, -1001000000060, 760003, 9299000001, 1794927780, 'poll', '', NULL, 'messagePoll', '{"poll_id":"5550101"}', NULL, 0),
	(6004, -1001000000060, 760004, 9299000004, 1794927840, 'checklist', 'Launch', NULL, 'messageChecklist', '{"tasks":[{"id":1,"text":"Review #42","entities":[{"offset":7,"length":3,"type":"hashtag"}]},{"id":2,"text":"Ask Dewi","entities":[{"offset":4,"length":4,"type":"mention_name","user_id":9299000001}]},{"id":3,"text":"Plain"}]}', NULL, 0),
	(6005, -1001000000060, 760005, 9299000001, 1794927900, 'service', 'pinned a message', '[{"offset":0,"length":6,"type":"bold"}]', 'messagePinMessage', '{"message_id":760001}', 'pin_message', 0),
	(6006, -1001000000060, 760006, 9299000004, 1794927960, 'text', 'Not an array', '{"offset":1}', 'messageText', NULL, NULL, 0),
	(6007, -1001000000060, 760007, 9299000001, 1794928020, 'text', 'No type here', '[{"offset":0,"length":2,"type":"bold"},{"offset":0,"length":3}]', 'messageText', NULL, NULL, 0),
	(6008, -1001000000060, 760008, 9299000004, 1794928080, 'text', 'Plain text', NULL, 'messageText', NULL, NULL, 0);

-- Message 2's version before its edit.
INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, entities,
	edit_date) VALUES
	(6002, 'text', 'Draft for @dewikusuma', '[{"offset":10,"length":11,"type":"mention"}]', 0);

INSERT INTO telegram_polls (id, question, question_entities, type, is_anonymous,
	allows_multiple_answers, correct_option_ids, explanation, explanation_entities,
	close_date, is_closed, total_voter_count) VALUES
	(5550101, 'Ship v2?', '[{"offset":5,"length":2,"type":"italic"}]', 'quiz', 1, 0, '[0]', 'Ask Bima first.',
	 '[{"offset":4,"length":4,"type":"mention_name","user_id":9299000002}]', 0, 0, 3);

INSERT INTO telegram_poll_options (poll_id, option_id, position, text, voter_count,
	vote_percentage) VALUES
	(5550101, '0', 0, 'Yes', 2, 67),
	(5550101, '1', 1, 'No', 1, 33);

INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing, date,
	edit_date, content_type, text, entities) VALUES
	(6101, 9299000001, 760101, 9299000001, 0, 1794928140, 0, 'text', 'see https://x.org',
	 '[{"offset":4,"length":13,"type":"url"}]');
