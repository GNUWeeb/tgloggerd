-- Dummy files that play a role in a message beside its own file (the
-- files list's role tests), loaded after "sample" (its users are the
-- senders and peers). Invented; nothing here comes from a real archive.
--
-- Ids, beside the sample's:
--   groups     -1001000000060 (Role Lab, supergroup, for a member),
--              -1001000000061 (Role Vault, supergroup, never granted)
--   files      891001 - 891010
--   messages   group rows 5101 - 5105 (Role Lab, n = 1 - 5) and 5111
--              (Role Vault); private rows 5201 - 5202 (with 9299000002)
-- By file:
--   891001  the picture of 5101's link preview (a text message)
--   891002  5102's own video
--   891003  5102's cover
--   891004  5102's still thumbnail
--   891005  the photo of 5201's game (incoming, private)
--   891006  5103's own photo, and the picture of 5104's link preview
--   891007  the cover of Role Vault's video 5111
--   891008  5111's own video
--   891009  the picture of 5202's link preview (outgoing, private)
--   891010  5105's own document, and a paid item of 5111

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at)
WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 10)
SELECT 891000 + n, CONCAT('role-', 891000 + n), CONCAT('role-u-', 891000 + n),
	ELT(n, 'photo', 'video', 'photo', 'thumbnail', 'photo', 'photo', 'photo', 'video',
	 'photo', 'document'),
	1000 * n, 1, UNHEX(SHA2(CONCAT('role ', 891000 + n), 256)),
	ELT(n, 'jpg', 'mp4', 'jpg', 'jpg', 'jpg', 'jpg', 'jpg', 'mp4', 'jpg', 'pdf'), '',
	'2026-11-17 10:00:00', '2026-11-17 10:00:00'
FROM seq;

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000060, 'supergroup', 'Role Lab', 'Files in their roles.', 5,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00'),
	(-1001000000061, 'supergroup', 'Role Vault', 'A group nobody is granted.', 1,
	 '2025-05-05 05:05:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, file_id) VALUES
	(5101, -1001000000060, 720001 << 20, 9299000001, 1794931200 - 600, 'text',
	 'see https://example.org/roles', NULL),
	(5102, -1001000000060, 720002 << 20, 9299000001, 1794931200 - 540, 'video', NULL, 891002),
	(5103, -1001000000060, 720003 << 20, 9299000001, 1794931200 - 480, 'photo', NULL, 891006),
	(5104, -1001000000060, 720004 << 20, 9299000001, 1794931200 - 420, 'text',
	 'again https://example.org/same', NULL),
	(5105, -1001000000060, 720005 << 20, 9299000001, 1794931200 - 360, 'document', NULL,
	 891010),
	(5111, -1001000000061, 721001 << 20, 9299000001, 1794931200 - 300, 'video', NULL, 891008);

INSERT INTO telegram_private_messages (id, chat_id, message_id, sender_id, is_outgoing, date,
	content_type, text, file_id) VALUES
	(5201, 9299000002, 630001 << 20, 9299000002, 0, 1794931200 - 240, 'game', NULL, NULL),
	(5202, 9299000002, 630002 << 20, NULL, 1, 1794931200 - 180, 'text',
	 'https://example.org/private', NULL);

INSERT INTO telegram_message_files (group_message_id, private_message_id, role, position,
	file_id) VALUES
	(5102, NULL, 'cover', 0, 891003),
	(5102, NULL, 'thumbnail', 0, 891004),
	(NULL, 5201, 'game', 0, 891005),
	(5111, NULL, 'cover', 0, 891007),
	(5111, NULL, 'paid', 0, 891010);

INSERT INTO telegram_link_previews (group_message_id, private_message_id, url, display_url,
	site_name, title, description, type, photo_file_id) VALUES
	(5101, NULL, 'https://example.org/roles', 'example.org/roles', 'Example', 'Roles',
	 'Files in their roles.', 'article', 891001),
	(5104, NULL, 'https://example.org/same', 'example.org/same', 'Example', 'Same',
	 'The same picture again.', 'photo', 891006),
	(NULL, 5202, 'https://example.org/private', 'example.org/private', 'Example', 'Private',
	 'A preview in a private chat.', 'article', 891009);
