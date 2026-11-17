-- Dummy web accounts, invented for tests and for the web app's
-- stand-in, whose sample accounts they follow
-- (web/mockup/src/data/accounts.js). Every password is "password". The
-- stand-in's superadmins are admins here and its members viewers,
-- until the schema has those roles; intern2025 is disabled.
--
-- Account ids 1 - 7; a test's own accounts get the next ids.

INSERT INTO web_users (id, username, password_hash, role, is_active, session_epoch,
	created_at, updated_at) VALUES
	(1, 'admin',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2025-07-25 16:00:00', '2025-07-25 16:00:00'),
	(2, 'ops',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2025-08-04 16:00:00', '2025-08-04 16:00:00'),
	(3, 'gw_admin',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2026-05-01 16:00:00', '2026-05-01 16:00:00'),
	(4, 'sec_lead',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2026-08-19 16:00:00', '2026-08-19 16:00:00'),
	(5, 'researcher',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'viewer', 1, 0, '2026-09-18 16:00:00', '2026-09-18 16:00:00'),
	(6, 'mod_kernel',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'viewer', 1, 0, '2026-11-14 16:00:00', '2026-11-14 16:00:00'),
	(7, 'intern2025',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'viewer', 0, 1, '2025-10-13 16:00:00', '2026-06-20 16:00:00');

INSERT INTO web_audit (web_user_id, action, ip, detail, created_at) VALUES
	(1, 'login_ok', '203.0.113.24', 'admin', '2026-11-17 15:58:00'),
	(NULL, 'login_fail', '198.51.100.7', 'admin', '2026-11-17 15:57:30'),
	(5, 'login_ok', '192.0.2.81', 'researcher', '2026-11-17 11:00:00'),
	(5, 'media', '192.0.2.81', 'file 880001', '2026-11-17 11:01:00'),
	(5, 'logout', '192.0.2.81', 'researcher', '2026-11-17 11:30:00');
