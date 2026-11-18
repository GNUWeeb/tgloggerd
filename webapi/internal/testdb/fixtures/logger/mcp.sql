-- Dummy data for the MCP endpoint's tests (internal/mcp, and
-- internal/server/mcp_test.go): two groups exposed through MCP and two
-- that are not, each with messages, so a test can check that nothing
-- of the unexposed ones is ever answered. Invented; nothing here comes
-- from a real archive. Loaded on its own, not beside "sample".
--
-- Ids:
--   users      9300000001 - 9300000005
--   groups     -1002000000001 Kernel Hackers ID  exposed
--              -1002000000002 Rust Belajar       exposed
--              -1002000000003 Secret Ops         NOT exposed (it has a
--                             public username: a hint, never a gate)
--              -1002000000004 Keluarga           NOT exposed
--   files      770001 - 770007
--   messages   row ids 5001 - 5301; Telegram message ids are server ids
--              shifted left by 20
-- Message dates are 1794700000 (2026-11-14T23:46:40Z) plus a few
-- hundred seconds, each one distinct, and so are the counts the tools
-- order by (messages per group, per sender, per user), so every
-- ordering is total and an answer is the same on every run.

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(770001, 'AgACAgUAAxkBAAIfmcp01', 'AQADmcp01', 'photo', 51200, 1,
	 UNHEX(SHA2('mcp group photo 770001', 256)), 'jpg', '',
	 '2026-03-01 00:00:00', '2026-11-15 00:00:00'),
	(770002, 'AgACAgUAAxkBAAIfmcp02', 'AQADmcp02', 'photo', 40960, 1,
	 UNHEX(SHA2('mcp profile photo 770002', 256)), 'jpg', '',
	 '2025-05-01 00:00:00', '2026-11-15 00:00:00'),
	(770003, 'AgACAgUAAxkBAAIfmcp03', 'AQADmcp03', 'photo', 30720, 0,
	 NULL, 'jpg', '', '2024-01-01 00:00:00', '2024-01-01 00:00:00'),
	(770004, 'BQACAgUAAxkBAAIfmcp04', 'AgADmcp04', 'document', 204800, 1,
	 UNHEX(SHA2('mcp document 770004', 256)), 'pdf', 'patch-v2.pdf',
	 '2026-11-15 01:51:40', '2026-11-15 01:51:40'),
	(770005, 'BAACAgUAAxkBAAIfmcp05', 'AgADmcp05', 'video', 52428800, 0,
	 NULL, 'mp4', 'demo.mp4', '2026-11-15 01:58:20', '2026-11-15 01:58:20'),
	(770006, 'AgACAgUAAxkBAAIfmcp06', 'AQADmcp06', 'photo', 81920, 1,
	 UNHEX(SHA2('mcp preview photo 770006', 256)), 'jpg', '',
	 '2026-11-15 02:03:20', '2026-11-15 02:03:20'),
	(770007, 'BQACAgUAAxkBAAIfmcp07', 'AgADmcp07', 'document', 102400, 1,
	 UNHEX(SHA2('mcp document 770007', 256)), 'pdf', 'patch-v1.pdf',
	 '2026-11-15 01:56:40', '2026-11-15 01:56:40');

INSERT INTO telegram_users (id, first_name, last_name, type, profile_photo_file_id,
	accent_color_id, is_verified, is_scam, is_fake, is_premium, is_support,
	birthday_day, birthday_month, birthday_year, msg_count, created_at, updated_at) VALUES
	(9300000001, 'Rina', 'Hartono', 'regular', 770002, 3, 1, 0, 0, 1, 0,
	 17, 8, NULL, 7, '2024-01-01 00:00:00', '2026-11-15 02:10:00'),
	(9300000002, 'Tono', 'Saputra', 'regular', NULL, 0, 0, 0, 0, 0, 0,
	 NULL, NULL, NULL, 6, '2024-06-01 00:00:00', '2026-11-15 02:05:00'),
	(9300000003, 'CI Bot', '', 'bot', NULL, 0, 0, 0, 0, 0, 0,
	 NULL, NULL, NULL, 2, '2025-01-01 00:00:00', '2026-11-15 02:05:00'),
	(9300000004, '', '', 'deleted', NULL, 0, 0, 0, 0, 0, 0,
	 NULL, NULL, NULL, 0, '2023-01-01 00:00:00', '2025-01-01 00:00:00'),
	(9300000005, 'Maya', 'Sari', 'regular', 770003, 0, 0, 0, 0, 0, 0,
	 2, 3, 1999, 3, '2025-03-01 00:00:00', '2026-11-15 02:00:00');

INSERT INTO telegram_user_extra_info (user_id, bio, phone_number, emoji_status_custom_emoji_id,
	restriction_reason, has_sensitive_content, restricts_new_chats,
	paid_message_star_count, language_code, personal_chat_id, created_at, updated_at) VALUES
	(9300000001, 'Kernel tinkerer', '6281200000001', 5368324170671202286, '', 0, 1,
	 0, 'id', NULL, '2025-02-01 00:00:00', '2025-02-01 00:00:00'),
	(9300000002, '', '', NULL, '', 0, 0, 25, 'en', NULL,
	 '2025-02-01 00:00:00', '2025-02-01 00:00:00');

INSERT INTO telegram_user_usernames (user_id, username, kind, is_collectible, position) VALUES
	(9300000001, 'rina_h', 'active', 0, 0),
	(9300000001, 'rinadev', 'active', 1, 1),
	(9300000001, 'rina_old', 'disabled', 0, 0),
	(9300000002, 'tono', 'active', 0, 0),
	(9300000003, 'gw_ci_bot', 'active', 0, 0);

INSERT INTO telegram_groups (id, type, title, description, photo_file_id, msg_count,
	created_at, updated_at) VALUES
	(-1002000000001, 'supergroup', 'Kernel Hackers ID',
	 'Linux kernel development, in Indonesian and English', 770001, 13,
	 '2025-01-01 00:00:00', '2026-11-15 02:10:00'),
	(-1002000000002, 'supergroup', 'Rust Belajar', '', NULL, 3,
	 '2025-04-01 00:00:00', '2026-11-15 01:55:00'),
	(-1002000000003, 'supergroup', 'Secret Ops', 'not for MCP', 770001, 2,
	 '2025-02-01 00:00:00', '2026-11-15 01:55:00'),
	(-1002000000004, 'basic_group', 'Keluarga', '', NULL, 1,
	 '2025-03-01 00:00:00', '2026-11-15 01:55:00');

INSERT INTO telegram_public_groups (group_id, note, added_by, created_at) VALUES
	(-1002000000001, 'kernel community', 1, '2026-01-01 00:00:00'),
	(-1002000000002, '', 1, '2026-01-02 00:00:00');

INSERT INTO telegram_group_usernames (group_id, username, kind, is_collectible, position) VALUES
	(-1002000000001, 'kernelhackers_id', 'active', 0, 0),
	(-1002000000001, 'kernel_id', 'active', 1, 1),
	(-1002000000001, 'oldkernel', 'disabled', 0, 0),
	(-1002000000003, 'secretops', 'active', 0, 0);

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, is_channel_post,
	date, edit_date, content_type, text, file_id, deleted_at, is_forwarded,
	reply_to_msg_id) VALUES
	(5001, -1002000000001, 101 << 20, 9300000001, 0, 1794700100, 0, 'text',
	 'Kernel panic on io_uring submit path, anyone seen this? https://lore.kernel.org/io-uring/20261114.abc@kernel.dk',
	 NULL, NULL, 0, NULL),
	(5002, -1002000000001, 102 << 20, 9300000002, 0, 1794700200, 0, 'text',
	 'yesss the kernel panic happens on 6.12 too wkwkwk', NULL, NULL, 0, 101 << 20),
	(5003, -1002000000001, 103 << 20, 9300000001, 0, 1794700300, 1794700900, 'document',
	 'patch v2 attached, fixes the kernel panic\nTested on two boxes', 770004, NULL, 0, NULL),
	(5004, -1002000000001, 104 << 20, 9300000003, 0, 1794700400, 0, 'text',
	 'Build #42 passed: kernel tree green. From: ci@example.org', NULL, NULL, 0, NULL),
	(5005, -1002000000001, 105 << 20, 9300000002, 0, 1794700500, 0, 'text',
	 'oops wrong group, ignore', NULL, '2026-11-15 10:00:00', 0, NULL),
	(5006, -1002000000001, 106 << 20, 9300000001, 0, 1794700600, 1794700700, 'text',
	 'Meetup notes: scheduler and memory management', NULL, NULL, 0, NULL),
	(5007, -1002000000001, 107 << 20, 9300000005, 0, 1794700700, 0, 'video',
	 '', 770005, NULL, 0, NULL),
	(5008, -1002000000001, 108 << 20, 9300000002, 0, 1794700800, 0, 'service',
	 'Tono Saputra joined the group', NULL, NULL, 0, NULL),
	(5009, -1002000000001, 109 << 20, NULL, 1, 1794700900, 0, 'text',
	 'Announcement: kernel meetup Saturday in Jakarta', NULL, NULL, 0, NULL),
	(5010, -1002000000001, 110 << 20, 9300000002, 0, 1794701000, 0, 'text',
	 'kernel 6.13 released, see https://kernel.org', NULL, NULL, 1, NULL),
	(5011, -1002000000001, 111 << 20, 9300000003, 0, 1794701100, 0, 'text',
	 'Build #43 passed: kernel tree green', NULL, NULL, 0, NULL),
	(5012, -1002000000001, 112 << 20, 9300000001, 0, 1794701200, 0, 'text',
	 'Thanks everyone, the patch is merged now', NULL, NULL, 0, NULL),
	(5013, -1002000000001, 113 << 20, 9300000001, 0, 1794701300, 0, 'text',
	 'rust or kernel, both fine for the scheduler talk', NULL, NULL, 0, NULL),
	(5101, -1002000000002, 201 << 20, 9300000001, 0, 1794700150, 0, 'text',
	 'Rust borrow checker question about lifetimes', NULL, NULL, 0, NULL),
	(5102, -1002000000002, 202 << 20, 9300000002, 0, 1794700250, 0, 'text',
	 'use Rc RefCell or rethink ownership', NULL, NULL, 0, NULL),
	(5103, -1002000000002, 203 << 20, 9300000005, 0, 1794700350, 0, 'text',
	 'kernel modules in rust are coming', NULL, NULL, 0, NULL),
	(5201, -1002000000003, 301 << 20, 9300000001, 0, 1794700160, 1794700170, 'text',
	 'secret kernel plan, do not share', NULL, NULL, 0, NULL),
	(5202, -1002000000003, 302 << 20, 9300000002, 0, 1794700260, 0, 'text',
	 'kernel kernel kernel', NULL, '2026-11-15 11:00:00', 0, NULL),
	(5301, -1002000000004, 401 << 20, 9300000005, 0, 1794700170, 0, 'text',
	 'family dinner, kernel talk banned', NULL, NULL, 0, NULL);

INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, file_id,
	edit_date, created_at) VALUES
	(5003, 'document', 'patch attached\nTested on one box', NULL, 0,
	 '2026-11-15 01:55:00'),
	(5003, 'document', 'patch v2 attached\nTested on one box', 770007, 1794700600,
	 '2026-11-15 02:00:00'),
	(5201, 'text', 'secret plan', NULL, 0, '2026-11-15 01:50:00');

INSERT INTO telegram_link_previews (group_message_id, url, display_url, site_name, title,
	description, author, type, instant_view_version, photo_file_id) VALUES
	(5001, 'https://lore.kernel.org/io-uring/20261114.abc@kernel.dk',
	 'lore.kernel.org/io-uring/20261114.abc@kernel.dk', 'lore.kernel.org',
	 'io_uring: fix submit path', 'The submit path dropped a reference.', '',
	 'article', 0, NULL),
	(5010, 'https://kernel.org', 'kernel.org', 'kernel.org', 'The Linux Kernel Archives',
	 '', 'kernel.org', 'photo', 2, 770006),
	(5201, 'https://secret.example', 'secret.example', 'secret', 'Secret', '', '',
	 'article', 0, NULL);

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title, inviter_user_id,
	joined_date, can_manage_chat, can_change_info, can_post_messages, can_edit_messages,
	can_delete_messages, can_invite_users, can_restrict_members, can_pin_messages,
	can_manage_topics, can_promote_members, can_manage_video_chats, is_anonymous) VALUES
	(-1002000000001, 9300000001, 'creator', 'Founder', 0, 1794600000,
	 1, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 0),
	(-1002000000001, 9300000002, 'administrator', '', 9300000001, 1794650000,
	 1, 0, 0, 0, 1, 0, 1, 1, 0, 0, 0, 0),
	(-1002000000001, 9300000003, 'administrator', 'CI', 9300000002, 1794680000,
	 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1),
	(-1002000000003, 9300000002, 'creator', 'Boss', 0, 1794600000,
	 1, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 0);

INSERT INTO telegram_group_hist_title (group_id, title, created_at) VALUES
	(-1002000000001, 'Kernel Hackers', '2025-01-01 00:00:00'),
	(-1002000000001, 'Kernel Hackers ID', '2026-03-01 00:00:00'),
	(-1002000000003, 'Secret Ops', '2025-02-01 00:00:00');

INSERT INTO telegram_group_hist_description (group_id, description, created_at) VALUES
	(-1002000000001, 'Linux kernel development, in Indonesian and English',
	 '2025-01-01 00:00:00');

INSERT INTO telegram_group_hist_usernames_events (group_id, username, action, kind,
	is_collectible, position, created_at) VALUES
	(-1002000000001, 'oldkernel', 'added', 'active', 0, 0, '2025-01-01 00:00:00'),
	(-1002000000001, 'kernelhackers_id', 'added', 'active', 0, 0, '2026-03-01 00:00:00'),
	(-1002000000001, 'oldkernel', 'kind_changed', 'disabled', 0, 0, '2026-03-01 00:00:01');

INSERT INTO telegram_group_hist_photo (group_id, file_id, created_at) VALUES
	(-1002000000001, 770003, '2025-01-01 00:00:00'),
	(-1002000000001, 770001, '2026-03-01 00:00:00');

INSERT INTO telegram_group_admin_hist (group_id, user_id, action, status, custom_title,
	is_anonymous, created_at) VALUES
	(-1002000000001, 9300000002, 'added', 'administrator', '', 0, '2025-06-01 00:00:00'),
	(-1002000000001, 9300000003, 'added', 'administrator', 'CI', 0, '2026-01-01 00:00:00'),
	(-1002000000001, 9300000003, 'updated', 'administrator', 'CI', 1, '2026-02-01 00:00:00');

INSERT INTO telegram_user_hist_name (user_id, first_name, last_name, created_at) VALUES
	(9300000001, 'Rina', 'H', '2024-01-01 00:00:00'),
	(9300000001, 'Rina', 'Hartono', '2025-01-01 00:00:00');

INSERT INTO telegram_user_hist_usernames_events (user_id, username, action, kind,
	is_collectible, position, created_at) VALUES
	(9300000001, 'rina_h', 'added', 'active', 0, 0, '2024-01-01 00:00:00'),
	(9300000001, 'rinadev', 'added', 'active', 1, 1, '2025-01-01 00:00:00');

INSERT INTO telegram_user_hist_bio (user_id, bio, created_at) VALUES
	(9300000001, 'Kernel tinkerer', '2025-02-01 00:00:00');

INSERT INTO telegram_user_hist_phone_num (user_id, phone_number, created_at) VALUES
	(9300000001, '6281200000001', '2025-02-01 00:00:00');

INSERT INTO telegram_user_hist_profile_photo (user_id, file_id, created_at) VALUES
	(9300000001, 770003, '2024-01-01 00:00:00'),
	(9300000001, 770002, '2025-05-01 00:00:00');
