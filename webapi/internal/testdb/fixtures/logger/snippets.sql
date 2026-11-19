-- Dummy messages without text of every kind that has a summary, and a
-- reply to each, for the summaries that stand in for a message without
-- text in reply quotes, the message lists and the dashboard
-- (webapi/internal/server/snippets.go). Loaded after "sample", whose
-- users are the senders; C/C++ Study Group (-1001000000004) posts the
-- story. Invented; nothing here comes from a real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000070 (Snippets Lab, not exposed)
--   files      895001 - 895002
--   polls      5570001 - 5570002
--   messages   group rows 7001 - 7020 (the kinds, n = 1 - 20) and
--              7101 - 7120 (reply n to message n)
-- Message n has message id 770000 + n, reply n 770100 + n; dates are
-- 1794931200 - 7200 + 60 * n and 1794931200 - 3600 + 60 * n. Dewi sends
-- them all. By n:
--   1  poll 5570001            2  quiz 5570002
--   3  venue, deleted          4  location
--   5  live location           6  contact
--   7  missed video call       8  voice call, hung up after 83 s
--   9  active group call      10  dart, 6
--  11  game                   12  invoice
--  13  story of C/C++ Study Group
--  14  giveaway of Premium with an extra prize
--  15  document report.pdf    16  photo without a name
--  17  checklist (no summary) 18  venue logged before migration 000034
--  19  giveaway of Stars      20  poll with a text

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(895001, 'snip-895001', 'snip-u-895001', 'document', 1000, 1,
	 UNHEX(SHA2('snip 895001', 256)), 'pdf', 'report.pdf', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(895002, 'snip-895002', 'snip-u-895002', 'photo', 2000, 1,
	 UNHEX(SHA2('snip 895002', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000070, 'supergroup', 'Snippets Lab', 'Messages without text.', 40,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title) VALUES
	(-1001000000070, 9299000001, 'administrator', '');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra, file_id) VALUES
	(7001, -1001000000070, 770001, 9299000001, 1794924060, 'poll', NULL, 'messagePoll', '{"poll_id":"5570001"}', NULL),
	(7002, -1001000000070, 770002, 9299000001, 1794924120, 'poll', '', 'messagePoll', '{"poll_id":"5570002"}', NULL),
	(7003, -1001000000070, 770003, 9299000001, 1794924180, 'venue', NULL, 'messageVenue', '{"lat":-6.175,"lon":106.8272,"title":"Monas","address":"Gambir, Jakarta"}', NULL),
	(7004, -1001000000070, 770004, 9299000001, 1794924240, 'location', NULL, 'messageLocation', '{"lat":-6.2,"lon":106.8166}', NULL),
	(7005, -1001000000070, 770005, 9299000001, 1794924300, 'location', NULL, 'messageLiveLocation', '{"lat":-6.21,"lon":106.82,"live":true,"live_period":900}', NULL),
	(7006, -1001000000070, 770006, 9299000001, 1794924360, 'contact', NULL, 'messageContact', '{"phone":"+628123456789","first_name":"Bima","last_name":"Pratama","user_id":9299000002}', NULL),
	(7007, -1001000000070, 770007, 9299000001, 1794924420, 'call', NULL, 'messageCall', '{"call_id":"201","video":true,"duration":0,"discard_reason":"missed"}', NULL),
	(7008, -1001000000070, 770008, 9299000001, 1794924480, 'call', NULL, 'messageCall', '{"call_id":"202","duration":83,"discard_reason":"hung_up"}', NULL),
	(7009, -1001000000070, 770009, 9299000001, 1794924540, 'call', NULL, 'messageGroupCall', '{"call_id":"203","group":true,"active":true}', NULL),
	(7010, -1001000000070, 770010, 9299000001, 1794924600, 'dice', NULL, 'messageDice', '{"emoji":"\\ud83c\\udfaf","value":6}', NULL),
	(7011, -1001000000070, 770011, 9299000001, 1794924660, 'game', NULL, 'messageGame', '{"short_name":"tetris","title":"Tetris"}', NULL),
	(7012, -1001000000070, 770012, 9299000001, 1794924720, 'invoice', NULL, 'messageInvoice', '{"title":"T-shirt","currency":"IDR","total_amount":15000000}', NULL),
	(7013, -1001000000070, 770013, 9299000001, 1794924780, 'story', NULL, 'messageStory', '{"poster_chat_id":-1001000000004,"story_id":12}', NULL),
	(7014, -1001000000070, 770014, 9299000001, 1794924840, 'giveaway', NULL, 'messageGiveaway', '{"boosted_chat_id":-1001000000070,"prize_description":"Stickers","winner_count":3,"prize":{"premium_months":6}}', NULL),
	(7015, -1001000000070, 770015, 9299000001, 1794924900, 'document', NULL, 'messageDocument', NULL, 895001),
	(7016, -1001000000070, 770016, 9299000001, 1794924960, 'photo', NULL, 'messagePhoto', NULL, 895002),
	(7017, -1001000000070, 770017, 9299000001, 1794925020, 'checklist', NULL, 'messageChecklist', '{"tasks":[{"id":1,"text":"Task 1"}]}', NULL),
	(7018, -1001000000070, 770018, 9299000001, 1794925080, 'venue', NULL, NULL, NULL, NULL),
	(7019, -1001000000070, 770019, 9299000001, 1794925140, 'giveaway', NULL, 'messageGiveaway', '{"boosted_chat_id":-1001000000070,"winner_count":1,"prize":{"stars":1000}}', NULL),
	(7020, -1001000000070, 770020, 9299000001, 1794925200, 'poll', 'Vote!', 'messagePoll', '{"poll_id":"5570001"}', NULL);

-- Reply n to message n.
INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, reply_to_msg_id, reply_to_id)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 20)
SELECT 7100 + n, -1001000000070, 770100 + n, 9299000001, 1794931200 - 3600 + 60 * n,
	'text', CONCAT('Re ', n), 770000 + n, 7000 + n FROM seq;

-- The venue was deleted after everything else the fixtures delete.
UPDATE telegram_group_messages SET deleted_at = '2026-11-17 16:59:00' WHERE id = 7003;

INSERT INTO telegram_polls (id, question, type, is_anonymous, allows_multiple_answers,
	correct_option_ids, explanation, close_date, is_closed, total_voter_count) VALUES
	(5570001, 'Lunch at noon?', 'regular', 0, 0, NULL, NULL, 0, 0, 2),
	(5570002, 'Capital of France?', 'quiz', 1, 0, '[0]', NULL, 0, 1, 5);

INSERT INTO telegram_poll_options (poll_id, option_id, position, text, voter_count,
	vote_percentage) VALUES
	(5570001, '0', 0, 'Yes', 1, 50),
	(5570001, '1', 1, 'No', 1, 50),
	(5570002, '0', 0, 'Paris', 5, 100);
