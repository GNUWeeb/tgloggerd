-- Dummy data for the logger's schema: a few users, groups, messages and
-- files. Invented for tests and for the web app's stand-in, whose
-- named samples it follows (web/mockup/src/data: Dewi Kusuma and the
-- three contacts, the named groups, GNU/Weeb, Rust Programming and CTF
-- Practice exposed). Nothing here comes from a real archive.
--
-- Ids, so a test can add rows of its own beside them:
--   users      9299000001 - 9299000008
--   groups     -1001000000001 - -1001000000010
--   files      880001 - 880008 (a test's own files: below 880000)
--   messages   row ids 1 - 99; Telegram message ids are server ids
--              shifted left by 20, as TDLib gives them
-- Times are around 2026-11-17T16:00:00Z, the stand-in's "now"
-- (1794931200).
--
-- Files marked on_disk with a sha256 have no blob in any store; a test
-- that serves one writes the blob itself (media.StorePath).

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	alias_of, sha256, file_ext, orig_file_name, hit_count, created_at, accessed_at) VALUES
	(880001, 'AgACAgUAAxkBAAIfixture01', 'AQADfixture01', 'photo', 183406, 1, NULL,
	 UNHEX(SHA2('fixture photo 880001', 256)), 'jpg', '', 12,
	 '2026-11-17 14:05:00', '2026-11-17 15:30:00'),
	(880002, 'BQACAgUAAxkBAAIfixture02', 'AgADfixture02', 'document', 3145728, 1, NULL,
	 UNHEX(SHA2('fixture document 880002', 256)), 'pdf', 'slides-io_uring.pdf', 4,
	 '2026-11-17 13:40:00', '2026-11-17 13:41:00'),
	(880003, 'BAACAgUAAxkBAAIfixture03', 'AgADfixture03', 'video', 52428800, 0, NULL,
	 NULL, 'mp4', 'demo.mp4', 1, '2026-11-16 20:00:00', '2026-11-16 20:00:00'),
	(880004, 'CAACAgUAAxkBAAIfixture04', 'AgADfixture04', 'sticker', 24576, 1, NULL,
	 UNHEX(SHA2('fixture sticker 880004', 256)), 'webp', '', 30,
	 '2026-11-17 15:10:00', '2026-11-17 15:55:00'),
	(880005, 'AwACAgUAAxkBAAIfixture05', 'AgADfixture05', 'voice', 48213, 0, NULL,
	 NULL, 'ogg', '', 1, '2026-11-17 09:12:00', '2026-11-17 09:12:00'),
	-- The same bytes as 880002, sent again under another name.
	(880006, 'BQACAgUAAxkBAAIfixture06', 'AgADfixture06', 'document', 3145728, 0, 880002,
	 NULL, 'pdf', 'io_uring-talk.pdf', 1, '2026-11-17 15:20:00', '2026-11-17 15:20:00'),
	(880007, 'AgACAgUAAxkBAAIfixture07', 'AQADfixture07', 'photo', 40960, 1, NULL,
	 UNHEX(SHA2('fixture profile photo 880007', 256)), 'jpg', '', 2,
	 '2026-10-01 08:00:00', '2026-11-17 12:00:00'),
	(880008, 'AgACAgUAAxkBAAIfixture08', 'AQADfixture08', 'photo', 51200, 1, NULL,
	 UNHEX(SHA2('fixture group photo 880008', 256)), 'jpg', '', 9,
	 '2026-09-12 10:00:00', '2026-11-17 15:00:00');

INSERT INTO telegram_users (id, first_name, last_name, is_contact, is_mutual_contact,
	contact_first_name, contact_last_name, type, profile_photo_file_id, is_verified,
	is_premium, msg_count, created_at, updated_at) VALUES
	(9299000001, 'Dewi', 'Kusuma', 1, 1, 'Dewi', '(kernel)', 'regular', 880007, 0, 1, 6,
	 '2023-02-11 07:30:00', '2026-11-17 15:31:00'),
	(9299000002, 'Bima', 'Pratama', 1, 0, 'Bima', 'kampus', 'regular', NULL, 0, 0, 3,
	 '2024-05-02 10:00:00', '2026-11-17 09:12:00'),
	(9299000003, 'Sari', 'Wulandari', 1, 1, 'Sari', 'GW', 'regular', NULL, 1, 1, 3,
	 '2022-08-19 12:00:00', '2026-11-17 15:40:00'),
	(9299000004, 'Andi', 'Santoso', NULL, 0, NULL, NULL, 'regular', NULL, 0, 0, 2,
	 '2025-01-04 18:20:00', '2026-11-16 21:00:00'),
	(9299000005, 'GW Build', '', NULL, 0, NULL, NULL, 'bot', NULL, 0, 0, 1,
	 '2024-11-30 00:00:00', '2026-11-17 14:00:00'),
	(9299000006, '', '', NULL, 0, NULL, NULL, 'deleted', NULL, 0, 0, 1,
	 '2023-06-01 09:00:00', '2026-03-01 09:00:00'),
	(9299000007, 'Putri', 'Lestari', NULL, 0, NULL, NULL, 'regular', NULL, 0, 1, 2,
	 '2025-07-21 16:45:00', '2026-11-17 13:42:00'),
	(9299000008, 'Fajar', 'Halim', NULL, 0, NULL, NULL, 'regular', NULL, 0, 0, 1,
	 '2026-11-10 11:11:00', '2026-11-10 11:11:00');

INSERT INTO telegram_user_usernames (user_id, username, kind, position) VALUES
	(9299000001, 'dewikusuma', 'active', 0),
	(9299000002, 'bima_pratama', 'active', 0),
	(9299000003, 'sariw', 'active', 0),
	(9299000003, 'sari_gw', 'disabled', 1),
	(9299000005, 'gwbuild_bot', 'active', 0),
	(9299000007, 'putri_l', 'active', 0),
	-- Released: no longer anyone's.
	(NULL, 'old_handle', 'active', 0);

INSERT INTO telegram_user_extra_info (user_id, bio, phone_number, language_code) VALUES
	(9299000001, 'Kernel hacker. Coffee first. She/her.', '+62 812 5550 1234', 'id'),
	(9299000003, 'Sysadmin by day, gamer by night', '', 'id'),
	(9299000007, 'Learning assembly the hard way', '', 'en');

INSERT INTO telegram_groups (id, type, title, description, photo_file_id, msg_count,
	created_at, updated_at) VALUES
	(-1001000000001, 'supergroup', 'GNU/Weeb',
	 'Low-level programming, Linux and free software.', 880008, 8,
	 '2022-03-14 09:00:00', '2026-11-17 15:40:00'),
	(-1001000000002, 'supergroup', 'Linux Kernel Indonesia',
	 'Discussing kernel development and patches.', NULL, 3,
	 '2022-06-01 09:00:00', '2026-11-17 13:42:00'),
	(-1001000000003, 'supergroup', 'Rust Programming',
	 'Rust questions, crates and code review.', NULL, 2,
	 '2023-01-20 09:00:00', '2026-11-16 21:00:00'),
	(-1001000000004, 'basic_group', 'C/C++ Study Group', 'Learn C and C++ together.', NULL, 0,
	 '2024-09-09 09:00:00', '2026-10-01 09:00:00'),
	(-1001000000005, 'channel', 'Assembly Lounge', 'x86-64, ARM64 and RISC-V assembly.',
	 NULL, 2, '2023-04-04 09:00:00', '2026-11-17 14:00:00'),
	(-1001000000010, 'supergroup', 'CTF Practice', 'Pwn, rev, crypto and web challenges.',
	 NULL, 1, '2024-02-02 09:00:00', '2026-11-15 19:00:00');

INSERT INTO telegram_group_usernames (group_id, username, kind, position) VALUES
	(-1001000000001, 'gnuweeb', 'active', 0),
	(-1001000000002, 'linuxkernelid', 'active', 0),
	(-1001000000003, 'rustlang_id', 'active', 0),
	(-1001000000005, 'asm_lounge', 'active', 0),
	(-1001000000010, 'ctfpractice', 'active', 0);

INSERT INTO telegram_group_admins (group_id, user_id, status, custom_title,
	can_manage_chat, can_delete_messages, can_restrict_members, can_pin_messages) VALUES
	(-1001000000001, 9299000003, 'creator', 'founder', 1, 1, 1, 1),
	(-1001000000001, 9299000001, 'administrator', '', 1, 1, 0, 1),
	(-1001000000002, 9299000001, 'administrator', 'maintainer', 1, 1, 1, 1),
	(-1001000000005, 9299000005, 'administrator', '', 1, 0, 0, 0);

-- Exposed to signed-out visitors and the MCP server.
INSERT INTO telegram_public_groups (group_id, note) VALUES
	(-1001000000001, 'community'),
	(-1001000000003, 'community'),
	(-1001000000010, 'practice');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, sender_chat_id,
	is_channel_post, author_signature, date, edit_date, content_type, text, service_type,
	file_id, deleted_at, is_forwarded, media_album_id, reply_to_chat_id, reply_to_msg_id,
	reply_to_id) VALUES
	(1, -1001000000001, 410001 << 20, 9299000003, NULL, 0, NULL, 1794931200 - 7200, 0,
	 'text', 'good morning everyone', NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL),
	(2, -1001000000001, 410002 << 20, 9299000001, NULL, 0, NULL, 1794931200 - 7100, 0,
	 'text', 'anyone tried the new kernel rc yet?', NULL, NULL, NULL, 0, NULL, NULL, NULL,
	 NULL),
	(3, -1001000000001, 410003 << 20, 9299000002, NULL, 0, NULL, 1794931200 - 7000,
	 1794931200 - 6900, 'text', 'the build is broken on arm64 again, see the log', NULL,
	 NULL, NULL, 0, NULL, -1001000000001, 410002 << 20, 2),
	(4, -1001000000001, 410004 << 20, 9299000001, NULL, 0, NULL, 1794931200 - 6800, 0,
	 'photo', 'flame graph from perf record -g', NULL, 880001, NULL, 0, NULL, NULL, NULL,
	 NULL),
	(5, -1001000000001, 410005 << 20, 9299000003, NULL, 0, NULL, 1794931200 - 3000, 0,
	 'document', 'slides from the io_uring talk', NULL, 880002, NULL, 0, NULL, NULL, NULL,
	 NULL),
	(6, -1001000000001, 410006 << 20, 9299000006, NULL, 0, NULL, 1794931200 - 2900, 0,
	 'text', 'this one was deleted', NULL, NULL, '2026-11-17 15:20:00', 0, NULL, NULL,
	 NULL, NULL),
	(7, -1001000000001, 410007 << 20, 9299000003, NULL, 0, NULL, 1794931200 - 2400, 0,
	 'sticker', '', NULL, 880004, NULL, 0, NULL, NULL, NULL, NULL),
	(8, -1001000000001, 410008 << 20, 9299000008, NULL, 0, NULL, 1794931200 - 1200, 0,
	 'service', NULL, 'messageChatJoinByLink', NULL, NULL, 0, NULL, NULL, NULL, NULL),
	(9, -1001000000002, 420001 << 20, 9299000001, NULL, 0, NULL, 1794931200 - 9000, 0,
	 'text', 'can someone review my patch? it is a two-liner', NULL, NULL, NULL, 0, NULL,
	 NULL, NULL, NULL),
	(10, -1001000000002, 420002 << 20, 9299000007, NULL, 0, NULL, 1794931200 - 8400, 0,
	 'text', 'try `git bisect run ./test.sh`, it finds it in 10 steps', NULL, NULL, NULL,
	 0, NULL, -1001000000002, 420001 << 20, 9),
	(11, -1001000000002, 420003 << 20, 9299000007, NULL, 0, NULL, 1794931200 - 8300, 0,
	 'document', 'forwarded the same slides', NULL, 880006, NULL, 1, NULL, NULL, NULL,
	 NULL),
	(12, -1001000000003, 430001 << 20, 9299000004, NULL, 0, NULL, 1794931200 - 68400, 0,
	 'text', 'is io_uring faster than epoll for this workload?', NULL, NULL, NULL, 0, NULL,
	 NULL, NULL, NULL),
	(13, -1001000000003, 430002 << 20, 9299000004, NULL, 0, NULL, 1794931200 - 68000, 0,
	 'video', 'benchmark run', NULL, 880003, NULL, 0, NULL, NULL, NULL, NULL),
	(14, -1001000000005, 450001 << 20, NULL, -1001000000005, 1, 'GW Build',
	 1794931200 - 9600, 0, 'text', 'x86-64 calling conventions, part 3', NULL, NULL, NULL,
	 0, NULL, NULL, NULL, NULL),
	(15, -1001000000005, 450002 << 20, 9299000005, -1001000000005, 1, NULL,
	 1794931200 - 7300, 0, 'text', 'nightly build ok', NULL, NULL, NULL, 0, NULL, NULL,
	 NULL, NULL),
	(16, -1001000000010, 500001 << 20, 9299000002, NULL, 0, NULL, 1794931200 - 162000, 0,
	 'text', 'pwn challenge 4 is up, flag format GW{...}', NULL, NULL, NULL, 0, NULL, NULL,
	 NULL, NULL);

-- Message 3's text before its edit.
INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, edit_date)
	VALUES (3, 'text', 'the build is broken on arm64 again', 1794931200 - 7000);

-- Private chats: chat_id is the other side's user id; outgoing messages
-- are the logging account's own.
INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing, date,
	edit_date, content_type, text, file_id, deleted_at, reply_to_chat_id, reply_to_msg_id,
	reply_to_id) VALUES
	(1, 9299000001, 610001 << 20, 9299000001, 0, 1794931200 - 5400, 0, 'text',
	 'Hari ini ada yang mau ngoding bareng?', NULL, NULL, NULL, NULL, NULL),
	(2, 9299000001, 610002 << 20, NULL, 1, 1794931200 - 5300, 0, 'text',
	 'sip, nanti malam', NULL, NULL, 9299000001, 610001 << 20, 1),
	(3, 9299000001, 610003 << 20, 9299000001, 0, 1794931200 - 1800, 0, 'text',
	 'meeting moved to 8pm', NULL, NULL, NULL, NULL, NULL),
	(4, 9299000001, 610004 << 20, 9299000001, 0, 1794931200 - 1700, 0, 'text',
	 'never mind', NULL, '2026-11-17 15:32:00', NULL, NULL, NULL),
	(5, 9299000002, 620001 << 20, 9299000002, 0, 1794931200 - 24480, 0, 'voice', NULL,
	 880005, NULL, NULL, NULL, NULL),
	(6, 9299000002, 620002 << 20, NULL, 1, 1794931200 - 24000, 0, 'text', 'thanks bang',
	 NULL, NULL, NULL, NULL, NULL);
