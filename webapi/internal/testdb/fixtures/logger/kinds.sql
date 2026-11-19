-- Dummy messages of every kind the logger stores since daemon migration
-- 000034 (docs/design/message-types.md section 4.2.2), for the chat's
-- message detail, service refs, interaction and topics. Loaded after
-- "sample", whose users are the senders and the people named. Invented;
-- nothing here comes from a real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000040 (Kinds Lab, a forum supergroup, not exposed)
--   files      870001 - 870004
--   polls      5550001 (a closed quiz of 25 options, one more removed)
--   messages   group rows 4001 - 4022 (Kinds Lab, n = 1 - 22)
-- Kinds Lab's message n has message id 700000 + n (a server id, as the
-- daemon stores them) and date 1794931200 - 3600 + 60 * n. Its senders
-- are Dewi (odd n) and Andi (even n) only, and Dewi is its admin, so a
-- member granted Kinds Lab sees those two and not Bima, whom several
-- messages name. C/C++ Study Group (-1001000000004) is named too. By n:
--   1  location            2  live location, moved 3 times (3 edits)
--   3  venue               4  contact (Bima, with phone and vCard)
--   5  dice                6  stake dice
--   7  game with a photo   8  poll 5550001, edited once (1 edit)
--   9  checklist of 22 tasks: 1 done by Bima, 2 by Dewi; it had 2
--      tasks before (1 edit), 1 done by Bima
--  10  story of C/C++ Study Group
--  11  invoice with a photo
--  12  giveaway: Kinds Lab, C/C++ Study Group, Linux Kernel Indonesia
--  13  video call, 83 s, hung up
--  14  group call of 25: Dewi, Bima, C/C++ Study Group, Andi and 21
--      users the archive has no row for
--  15  paid media: a photo (with its file) and a preview
--  16  rich message      17  expired media
--  18  service chat_add_members: Dewi added Andi and Bima
--  19  service giveaway_winners of C/C++ Study Group: Bima, Dewi
--  20  service pin_message of message 3
--  21  service chat_add_members logged before migration 000034 (no
--      tdlib_type, no content_extra)
--  22  service chat_delete_member: Dewi removed Bima
-- Its topics and the interaction totals are at the end.

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(870001, 'kinds-870001', 'kinds-u-870001', 'photo', 40000, 1,
	 UNHEX(SHA2('kinds 870001', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(870002, 'kinds-870002', 'kinds-u-870002', 'photo', 41000, 1,
	 UNHEX(SHA2('kinds 870002', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(870003, 'kinds-870003', 'kinds-u-870003', 'photo', 42000, 1,
	 UNHEX(SHA2('kinds 870003', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000040, 'supergroup', 'Kinds Lab', 'Every kind of message.', 22,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title) VALUES
	(-1001000000040, 9299000001, 'administrator', '');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra, service_type, file_id, edit_date) VALUES
	(4001, -1001000000040, 700001, 9299000001, 1794927660, 'location', NULL, 'messageLocation', '{"lat":-6.2,"lon":106.8166,"accuracy":5.5}', NULL, NULL, 0),
	(4002, -1001000000040, 700002, 9299000004, 1794927720, 'location', NULL, 'messageLiveLocation', '{"lat":-6.21,"lon":106.82,"live":true,"live_period":900,"heading":90,"proximity_radius":100}', NULL, NULL, 1794927900),
	(4003, -1001000000040, 700003, 9299000001, 1794927780, 'venue', NULL, 'messageVenue', '{"lat":-6.175,"lon":106.8272,"title":"Monas","address":"Gambir, Jakarta","provider":"foursquare","venue_id":"4b0587","venue_type":"arts_entertainment/monument"}', NULL, NULL, 0),
	(4004, -1001000000040, 700004, 9299000004, 1794927840, 'contact', NULL, 'messageContact', '{"phone":"+628123456789","first_name":"Bima","last_name":"Pratama","vcard":"BEGIN:VCARD\\nTEL:+628123456789\\nEND:VCARD","user_id":9299000002}', NULL, NULL, 0),
	(4005, -1001000000040, 700005, 9299000001, 1794927900, 'dice', NULL, 'messageDice', '{"emoji":"\\ud83c\\udfb2","value":4}', NULL, NULL, 0),
	(4006, -1001000000040, 700006, 9299000004, 1794927960, 'dice', NULL, 'messageStakeDice', '{"value":3,"stake_gram_amount":100,"stake":true}', NULL, NULL, 0),
	(4007, -1001000000040, 700007, 9299000001, 1794928020, 'game', 'Play it', 'messageGame', '{"game_id":"777","short_name":"tetris","title":"Tetris","description":"Falling blocks.","has_photo":true}', NULL, NULL, 0),
	(4008, -1001000000040, 700008, 9299000004, 1794928080, 'poll', 'Pick one', 'messagePoll', '{"poll_id":"5550001"}', NULL, NULL, 1794928110),
	(4009, -1001000000040, 700009, 9299000001, 1794928140, 'checklist', 'Release checklist', 'messageChecklist', '{"tasks":[{"id":1,"text":"Task 1","completed_by":{"user_id":9299000002},"completion_date":1794927700},{"id":2,"text":"Task 2","completed_by":{"user_id":9299000001},"completion_date":1794927800},{"id":3,"text":"Task 3"},{"id":4,"text":"Task 4"},{"id":5,"text":"Task 5"},{"id":6,"text":"Task 6"},{"id":7,"text":"Task 7"},{"id":8,"text":"Task 8"},{"id":9,"text":"Task 9"},{"id":10,"text":"Task 10"},{"id":11,"text":"Task 11"},{"id":12,"text":"Task 12"},{"id":13,"text":"Task 13"},{"id":14,"text":"Task 14"},{"id":15,"text":"Task 15"},{"id":16,"text":"Task 16"},{"id":17,"text":"Task 17"},{"id":18,"text":"Task 18"},{"id":19,"text":"Task 19"},{"id":20,"text":"Task 20"},{"id":21,"text":"Task 21"},{"id":22,"text":"Task 22"}],"others_can_add_tasks":true}', NULL, NULL, 1794928170),
	(4010, -1001000000040, 700010, 9299000004, 1794928200, 'story', NULL, 'messageStory', '{"poster_chat_id":-1001000000004,"story_id":12}', NULL, NULL, 0),
	(4011, -1001000000040, 700011, 9299000001, 1794928260, 'invoice', NULL, 'messageInvoice', '{"title":"T-shirt","description":{"text":"A black T-shirt."},"has_photo":true,"currency":"IDR","total_amount":15000000,"is_test":true}', NULL, NULL, 0),
	(4012, -1001000000040, 700012, 9299000004, 1794928320, 'giveaway', NULL, 'messageGiveaway', '{"boosted_chat_id":-1001000000040,"additional_chat_ids":[-1001000000004,-1001000000002],"winners_selection_date":1795014000,"only_new_members":true,"prize_description":"Stickers","winner_count":3,"prize":{"premium_months":6}}', NULL, NULL, 0),
	(4013, -1001000000040, 700013, 9299000001, 1794928380, 'call', NULL, 'messageCall', '{"call_id":"99","video":true,"duration":83,"discard_reason":"hung_up"}', NULL, NULL, 0),
	(4014, -1001000000040, 700014, 9299000004, 1794928440, 'call', NULL, 'messageGroupCall', '{"call_id":"100","group":true,"duration":3725,"participants":[{"user_id":9299000001},{"user_id":9299000002},{"chat_id":-1001000000004},{"user_id":9299000004},{"user_id":9299000101},{"user_id":9299000102},{"user_id":9299000103},{"user_id":9299000104},{"user_id":9299000105},{"user_id":9299000106},{"user_id":9299000107},{"user_id":9299000108},{"user_id":9299000109},{"user_id":9299000110},{"user_id":9299000111},{"user_id":9299000112},{"user_id":9299000113},{"user_id":9299000114},{"user_id":9299000115},{"user_id":9299000116},{"user_id":9299000117},{"user_id":9299000118},{"user_id":9299000119},{"user_id":9299000120},{"user_id":9299000121}]}', NULL, NULL, 0),
	(4015, -1001000000040, 700015, 9299000001, 1794928500, 'paid_media', 'Pay to see', 'messagePaidMedia', '{"star_count":50,"items":[{"kind":"photo","width":800,"height":600},{"kind":"preview","width":640,"height":480}]}', NULL, NULL, 0),
	(4016, -1001000000040, 700016, 9299000004, 1794928560, 'rich', NULL, 'messageRichMessage', '{"tdlib":{"@type":"messageRichMessage"}}', NULL, NULL, 0),
	(4017, -1001000000040, 700017, 9299000001, 1794928620, 'expired', NULL, 'messageExpiredPhoto', NULL, NULL, NULL, 0),
	(4018, -1001000000040, 700018, 9299000004, 1794928680, 'service', 'added 2 members to the group', 'messageChatAddMembers', '{"user_ids":[9299000004,9299000002]}', 'chat_add_members', NULL, 0),
	(4019, -1001000000040, 700019, 9299000001, 1794928740, 'service', 'announced the giveaway winners', 'messageGiveawayWinners', '{"boosted_chat_id":-1001000000004,"giveaway_message_id":700012,"winner_count":2,"winner_user_ids":[9299000002,9299000001]}', 'giveaway_winners', NULL, 0),
	(4020, -1001000000040, 700020, 9299000004, 1794928800, 'service', 'pinned a message', 'messagePinMessage', '{"message_id":700003}', 'pin_message', NULL, 0),
	(4021, -1001000000040, 700021, 9299000001, 1794928860, 'service', 'added a member to the group', NULL, NULL, 'chat_add_members', NULL, 0),
	(4022, -1001000000040, 700022, 9299000004, 1794928920, 'service', 'removed a member from the group', 'messageChatDeleteMember', '{"user_id":9299000002}', 'chat_delete_member', NULL, 0);

-- The live location's earlier positions, one version each.
INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, edit_date,
	content_extra) VALUES
	(4002, 'location', NULL, 0, '{"lat":-6.2,"lon":106.81,"live":true,"live_period":900}'),
	(4002, 'location', NULL, 1794931200 - 3600 + 120 + 60,
	 '{"lat":-6.205,"lon":106.815,"live":true,"live_period":900}'),
	(4002, 'location', NULL, 1794931200 - 3600 + 120 + 120,
	 '{"lat":-6.208,"lon":106.818,"live":true,"live_period":900}');

-- The poll's version before its question was edited (the poll tables
-- keep only the latest results) and the checklist's version before
-- tasks were added to it.
INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, edit_date,
	content_extra) VALUES
	(4008, 'poll', 'Pick', 0, '{"poll_id":"5550001"}'),
	(4009, 'checklist', 'Release checklist', 0,
	 '{"tasks":[{"id":1,"text":"Task 1","completed_by":{"user_id":9299000002},"completion_date":1794927700},{"id":2,"text":"Task 2"}]}');

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(4007, 'game', 0, 870001),
	(4011, 'invoice', 0, 870002),
	(4015, 'paid', 0, 870003);

INSERT INTO telegram_polls (id, question, type, is_anonymous, allows_multiple_answers,
	correct_option_ids, explanation, close_date, is_closed, total_voter_count) VALUES
	(5550001, 'Which option?', 'quiz', 1, 0, '[2]', 'The third one.', 1794931200, 1, 325);

INSERT INTO telegram_poll_options (poll_id, option_id, position, text, voter_count,
	vote_percentage)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 25)
SELECT 5550001, CONCAT('o', n), n - 1, CONCAT('Option ', n), n, 4 FROM seq;

INSERT INTO telegram_poll_options (poll_id, option_id, position, text, voter_count,
	vote_percentage, deleted_at) VALUES
	(5550001, 'gone', 25, 'Removed option', 0, 0, '2026-11-17 10:00:00');

-- Kinds Lab is a forum: General (1), Releases (5, pinned) and
-- Off-topic (9, closed). Messages 1 - 5 are in Releases, 6 in General;
-- the others name no topic, as rows logged before the daemon recorded
-- topics. A thread's id (topic_kind thread) is no forum topic.
INSERT INTO telegram_forum_topics (chat_id, topic_id, name, icon_color, creation_date,
	creator_user_id, is_general, is_closed, is_hidden, is_pinned) VALUES
	(-1001000000040, 1, 'General', 0, 1794900000, 9299000001, 1, 0, 1, 0),
	(-1001000000040, 5, 'Releases', 7322096, 1794910000, 9299000001, 0, 0, 0, 1),
	(-1001000000040, 9, 'Off-topic', 16749490, 1794920000, 9299000004, 0, 1, 0, 0);
UPDATE telegram_group_messages SET topic_kind = 'forum', topic_id = 5
	WHERE id BETWEEN 4001 AND 4005;
UPDATE telegram_group_messages SET topic_kind = 'forum', topic_id = 1 WHERE id = 4006;
UPDATE telegram_group_messages SET topic_kind = 'thread', topic_id = 5 WHERE id = 4007;

-- Interaction: message 1 was seen, forwarded, replied to and reacted
-- to (an emoji, a custom emoji, the paid reaction); message 3 is the
-- pinned one; message 2's reactions were all withdrawn.
UPDATE telegram_group_messages SET view_count = 120, forward_count = 3, reply_count = 2,
	reactions = '[{"emoji":"\\ud83d\\udc4d","count":5},{"custom_emoji_id":"5368324170671202286","count":2},{"paid":true,"count":1}]'
	WHERE id = 4001;
UPDATE telegram_group_messages SET is_pinned = 1 WHERE id = 4003;
UPDATE telegram_group_messages SET view_count = 0, reactions = '[]' WHERE id = 4002;
-- Message 1 was forwarded from a channel, whose post's counts it
-- carries; message 2 is the group's own, whose stored 0 views mean
-- nothing.
INSERT INTO telegram_group_message_fwd_info (group_message_id, origin_type, origin_sender_name)
	VALUES (4001, 'channel', 'Kinds News');
