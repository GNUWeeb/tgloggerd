-- Dummy people for the user and group pages: a user with every profile
-- field the logger records and a long change history, a contact whose
-- own name was never seen and whose real-name lookup is pending, and a
-- group with several usernames and a long history. Invented for tests;
-- loads after "sample" (it points at sample's files and groups).
--
-- Ids, so a test can add rows of its own beside them:
--   users      9299200001 (Persona Lengkap), 9299200002 (Kontak Saja)
--   groups     -1001000200001 (History Lab)
--   history    row ids 1 - 999 in every history table
--
-- Persona's timeline, 160 entries, newest first:
--   2026-06-01 12:00:00  six at the same second: bio, name "Tie B"
--                        (id 502), name "Tie A" (id 501), phone, photo,
--                        username "persona" added (the sort is the
--                        time, then the kind, then the row id, newest
--                        first)
--   2026-05-01 - 05-04   username events: reordered, kind_changed,
--                        collectible_changed, removed
--   2026-01-01 00:01 -   150 names "Name 1" to "Name 150", one a
--   2026-01-01 02:30     minute (ids 1 - 150)
-- History Lab's, 125 entries: at 2026-06-01 12:00:00 a description,
-- an admin added and a username added; an admin update (2026-05-02),
-- a photo removed (2026-05-03); 120 titles "Title 1" to "Title 120",
-- one a minute from 2026-01-01 00:01.

INSERT INTO telegram_users (id, first_name, last_name, type, profile_photo_file_id,
	accent_color_id, is_verified, is_premium, is_scam, is_fake, is_support,
	birthday_day, birthday_month, birthday_year, is_contact, is_mutual_contact,
	contact_first_name, contact_last_name, own_name_known, own_name_checked_at,
	msg_count, created_at, updated_at) VALUES
	(9299200001, 'Persona', 'Lengkap', 'regular', 880007, 5, 1, 1, 0, 0, 0,
	 17, 8, 1995, 1, 1, 'Persona', '(fixture)', 1, '2026-10-01 09:00:00',
	 0, '2025-12-01 08:00:00', '2026-11-10 10:00:00'),
	(9299200002, 'Kontak', 'Saja', 'regular', NULL, 0, 0, 0, 0, 0, 0,
	 3, 12, NULL, 1, 0, 'Kontak', 'Saja', 0, NULL,
	 0, '2025-12-02 08:00:00', '2026-11-10 10:00:00');

INSERT INTO telegram_user_extra_info (user_id, bio, phone_number, background_custom_emoji_id,
	profile_accent_color_id, profile_background_custom_emoji_id,
	emoji_status_custom_emoji_id, emoji_status_expiration_date, restriction_reason,
	has_sensitive_content, restricts_new_chats, paid_message_star_count, language_code,
	personal_chat_id) VALUES
	(9299200001, 'Every field, for tests.', '+62 800 0000 0001', 5368324170671202286,
	 3, 5370870893004203704, 5373141891321699086, 1798761600,
	 'Restricted on some platforms (fixture)', 1, 1, 250, 'id', -1001000000001);

INSERT INTO telegram_user_usernames (user_id, username, kind, position, is_collectible) VALUES
	(9299200001, 'persona', 'active', 0, 0),
	(9299200001, 'persona_nft', 'active', 1, 1),
	(9299200001, 'persona_old', 'disabled', 0, 0);

INSERT INTO telegram_contact_lookups (user_id, first_name, last_name, removed_at, attempts,
	last_error, created_at) VALUES
	(9299200002, 'Kontak', 'Saja', '2026-11-17 10:00:00', 2, 'FLOOD_WAIT_30 <retry>',
	 '2026-11-17 09:59:00');

INSERT INTO telegram_user_hist_name (id, user_id, first_name, last_name, created_at)
	WITH RECURSIVE n (i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 150)
	SELECT i, 9299200001, CONCAT('Name ', i), '',
	       TIMESTAMPADD(MINUTE, i, '2026-01-01 00:00:00') FROM n;

INSERT INTO telegram_user_hist_name (id, user_id, first_name, last_name, created_at) VALUES
	(501, 9299200001, 'Tie', 'A', '2026-06-01 12:00:00'),
	(502, 9299200001, 'Tie', 'B', '2026-06-01 12:00:00');

INSERT INTO telegram_user_hist_bio (id, user_id, bio, created_at) VALUES
	(1, 9299200001, 'Every field, for tests.', '2026-06-01 12:00:00');

INSERT INTO telegram_user_hist_phone_num (id, user_id, phone_number, created_at) VALUES
	(1, 9299200001, '+62 800 0000 0001', '2026-06-01 12:00:00');

INSERT INTO telegram_user_hist_profile_photo (id, user_id, file_id, created_at) VALUES
	(1, 9299200001, 880007, '2026-06-01 12:00:00');

INSERT INTO telegram_user_hist_usernames_events (id, user_id, username, action, kind,
	is_collectible, position, created_at) VALUES
	(1, 9299200001, 'persona_old', 'removed', NULL, NULL, NULL, '2026-05-01 10:00:00'),
	(2, 9299200001, 'persona_nft', 'collectible_changed', 'active', 1, 1, '2026-05-02 10:00:00'),
	(3, 9299200001, 'persona_old', 'kind_changed', 'disabled', 0, 0, '2026-05-03 10:00:00'),
	(4, 9299200001, 'persona_nft', 'reordered', 'active', 1, 1, '2026-05-04 10:00:00'),
	(5, 9299200001, 'persona', 'added', 'active', 0, 0, '2026-06-01 12:00:00');

INSERT INTO telegram_groups (id, type, title, description, photo_file_id, msg_count,
	created_at, updated_at) VALUES
	(-1001000200001, 'supergroup', 'History Lab', 'A group with a long history.', 880008, 0,
	 '2025-12-01 08:00:00', '2026-11-10 10:00:00');

INSERT INTO telegram_group_usernames (group_id, username, kind, position, is_collectible) VALUES
	(-1001000200001, 'historylab', 'active', 0, 0),
	(-1001000200001, 'historylab_nft', 'active', 1, 1),
	(-1001000200001, 'historylab_old', 'disabled', 0, 0);

INSERT INTO telegram_group_hist_title (id, group_id, title, created_at)
	WITH RECURSIVE n (i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 120)
	SELECT i, -1001000200001, CONCAT('Title ', i),
	       TIMESTAMPADD(MINUTE, i, '2026-01-01 00:00:00') FROM n;

INSERT INTO telegram_group_hist_description (id, group_id, description, created_at) VALUES
	(1, -1001000200001, 'A group with a long history.', '2026-06-01 12:00:00');

INSERT INTO telegram_group_hist_usernames_events (id, group_id, username, action, kind,
	is_collectible, position, created_at) VALUES
	(1, -1001000200001, 'historylab', 'added', 'active', 0, 0, '2026-06-01 12:00:00');

INSERT INTO telegram_group_hist_photo (id, group_id, file_id, created_at) VALUES
	(1, -1001000200001, NULL, '2026-05-03 10:00:00');

INSERT INTO telegram_group_admin_hist (id, group_id, user_id, action, status, custom_title,
	created_at) VALUES
	(1, -1001000200001, 9299000001, 'updated', 'creator', '', '2026-05-02 10:00:00'),
	(2, -1001000200001, 9299000003, 'added', 'administrator', 'keeper',
	 '2026-06-01 12:00:00');
