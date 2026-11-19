-- Service messages with a stored detail beyond their line
-- (docs/design/message-types.md section 4.2.3), for ServiceInfo.detail.
-- Loaded after "sample", whose users send them and whose groups are
-- named. Invented; nothing here comes from a real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000042 (Service Lab, a forum supergroup, not
--              exposed), -4242001 (its basic group before the upgrade)
--   files      875001 - 875002
--   messages   group rows 4201 - 4223 (Service Lab, n = 1 - 23)
--   communities 9000000001 - 9000000004 (TDLib community ids)
-- Service Lab's message n has message id 720000 + n (a server id, as
-- the daemon stores them) and date 1794931200 - 3600 + 60 * n; Dewi
-- sends them all. C/C++ Study Group (-1001000000004), which an admin
-- hides and a member granted Service Lab does not see, is a story's
-- poster and the one chat of community 9000000001. By n:
--   1  chat_change_photo with its photo
--   2  video_chat_ended after 3725 s     3  video_chat_scheduled
--   4  a text message                    5  pin_message of 4
--   6  a story of C/C++ Study Group      7  pin_message of 6
--   8  pin_message of a message the archive does not have
--   9  poll_option_added                10  poll_option_deleted
--  11  forum_topic_created (topic 11)   12  forum_topic_is_closed_toggled
--  13  chat_added_to_community of 9000000001 (C/C++ Study Group alone)
--  14  chat_join_from_community of 9000000002 (Service Lab and C/C++
--      Study Group, with its photo)
--  15  chat_removed_from_community (no community stored)
--  16  chat_create                      17  chat_upgrade_from -4242001
--  18  chat_set_background for the setter alone
--  19  chat_change_photo logged without its detail, with its photo
--  20  suggest_profile_photo, its photo not stored
--  21  chat_join_from_community of 9000000003 (no chats known yet)
--  22  chat_added_to_community of 9000000004 (no name known yet)
--  23  chat_added_to_community naming C/C++ Study Group's chat id, which
--      is no community id: the archive has no community for it

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(875001, 'svc-875001', 'svc-u-875001', 'photo', 30000, 1,
	 UNHEX(SHA2('svc 875001', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(875002, 'svc-875002', 'svc-u-875002', 'photo', 31000, 0,
	 NULL, 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000042, 'supergroup', 'Service Lab', 'Service lines.', 23,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00'),
	(-4242001, 'basic_group', 'Service Lab (basic)', '', 0,
	 '2025-01-01 00:00:00', '2025-06-06 06:06:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra, service_type) VALUES
	(4201, -1001000000042, 720001, 9299000001, 1794927660, 'service', 'changed the group photo', 'messageChatChangePhoto', '{"photo":{"photo_id":"31","width":640,"height":640}}', 'chat_change_photo'),
	(4202, -1001000000042, 720002, 9299000001, 1794927720, 'service', 'ended the video chat', 'messageVideoChatEnded', '{"duration":3725}', 'video_chat_ended'),
	(4203, -1001000000042, 720003, 9299000001, 1794927780, 'service', 'scheduled a video chat', 'messageVideoChatScheduled', '{"group_call_id":5,"start_date":1795000000}', 'video_chat_scheduled'),
	(4204, -1001000000042, 720004, 9299000001, 1794927840, 'text', 'Release notes for 1.0 are out', NULL, NULL, NULL),
	(4205, -1001000000042, 720005, 9299000001, 1794927900, 'service', 'pinned a message', 'messagePinMessage', '{"message_id":720004}', 'pin_message'),
	(4206, -1001000000042, 720006, 9299000001, 1794927960, 'story', NULL, 'messageStory', '{"poster_chat_id":-1001000000004,"story_id":7}', NULL),
	(4207, -1001000000042, 720007, 9299000001, 1794928020, 'service', 'pinned a message', 'messagePinMessage', '{"message_id":720006}', 'pin_message'),
	(4208, -1001000000042, 720008, 9299000001, 1794928080, 'service', 'pinned a message', 'messagePinMessage', '{"message_id":729999}', 'pin_message'),
	(4209, -1001000000042, 720009, 9299000001, 1794928140, 'service', 'added an option to a poll', 'messagePollOptionAdded', '{"poll_message_id":720004,"option_id":"c","option":{"text":"Option C"}}', 'poll_option_added'),
	(4210, -1001000000042, 720010, 9299000001, 1794928200, 'service', 'removed an option from a poll', 'messagePollOptionDeleted', '{"poll_message_id":720004,"option_id":"b","option":{"text":"Option B"}}', 'poll_option_deleted'),
	(4211, -1001000000042, 720011, 9299000001, 1794928260, 'service', 'created the topic', 'messageForumTopicCreated', '{"name":"Releases","icon_color":7322096}', 'forum_topic_created'),
	(4212, -1001000000042, 720012, 9299000001, 1794928320, 'service', 'closed the topic', 'messageForumTopicIsClosedToggled', '{"closed":true}', 'forum_topic_is_closed_toggled'),
	(4213, -1001000000042, 720013, 9299000001, 1794928380, 'service', 'added the group to a community', 'messageChatAddedToCommunity', '{"community_id":9000000001}', 'chat_added_to_community'),
	(4214, -1001000000042, 720014, 9299000001, 1794928440, 'service', 'joined the group from a community', 'messageChatJoinFromCommunity', '{"community_id":9000000002}', 'chat_join_from_community'),
	(4215, -1001000000042, 720015, 9299000001, 1794928500, 'service', 'removed the group from its community', 'messageChatRemovedFromCommunity', NULL, 'chat_removed_from_community'),
	(4216, -1001000000042, 720016, 9299000001, 1794928560, 'service', 'created the group', 'messageSupergroupChatCreate', '{"title":"Service Lab"}', 'chat_create'),
	(4217, -1001000000042, 720017, 9299000001, 1794928620, 'service', 'upgraded the group', 'messageChatUpgradeFrom', '{"title":"Service Lab (basic)","basic_group_id":4242001}', 'chat_upgrade_from'),
	(4218, -1001000000042, 720018, 9299000001, 1794928680, 'service', 'set a new background', 'messageChatSetBackground', '{"only_for_self":true}', 'chat_set_background'),
	(4219, -1001000000042, 720019, 9299000001, 1794928740, 'service', 'changed the group photo', NULL, NULL, 'chat_change_photo'),
	(4220, -1001000000042, 720020, 9299000001, 1794928800, 'service', 'suggested a profile photo', 'messageSuggestProfilePhoto', '{"photo":{"photo_id":"32","width":320,"height":320}}', 'suggest_profile_photo'),
	(4221, -1001000000042, 720021, 9299000001, 1794928860, 'service', 'joined the group from a community', 'messageChatJoinFromCommunity', '{"community_id":9000000003}', 'chat_join_from_community'),
	(4222, -1001000000042, 720022, 9299000001, 1794928920, 'service', 'added the group to a community', 'messageChatAddedToCommunity', '{"community_id":9000000004}', 'chat_added_to_community'),
	(4223, -1001000000042, 720023, 9299000001, 1794928980, 'service', 'added the group to a community', 'messageChatAddedToCommunity', '{"community_id":-1001000000004}', 'chat_added_to_community');

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(4201, 'chat_photo', 0, 875001),
	(4219, 'chat_photo', 0, 875001),
	(4220, 'chat_photo', 0, 875002);

-- Service Lab is a forum: the topic created by message 11 was renamed
-- since, and its closing is in it too.
INSERT INTO telegram_forum_topics (chat_id, topic_id, name, icon_color, creation_date,
	creator_user_id, is_general, is_closed, is_hidden, is_pinned) VALUES
	(-1001000000042, 11, 'Releases 2', 16749490, 1794928260, 9299000001, 0, 1, 0, 0);
UPDATE telegram_group_messages SET topic_kind = 'forum', topic_id = 11
	WHERE id IN (4211, 4212);

-- The communities the lines name (migration 000049), as updateCommunity
-- and their full info left them.
INSERT INTO telegram_communities (id, name, have_access, photo_file_id, date) VALUES
	(9000000001, 'Systems Programmers', 1, NULL, 1794900000),
	(9000000002, 'Open Source Lab', 1, 875001, 1794900000),
	(9000000003, 'New Community', 1, NULL, NULL),
	(9000000004, '', 0, NULL, NULL);
INSERT INTO telegram_community_chats (community_id, chat_id, is_hidden, can_view_history) VALUES
	(9000000001, -1001000000004, 0, 1),
	(9000000002, -1001000000004, 0, 1),
	(9000000002, -1001000000042, 1, 1),
	(9000000004, -1001000000042, 0, 1);
