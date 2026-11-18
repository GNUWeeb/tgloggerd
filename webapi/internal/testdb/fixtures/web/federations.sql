-- Dummy federations, invented for tests: two federations besides
-- Default over the groups of the logger's "sample" fixture, with
-- admins and members spread over them. Loads on its own (without the
-- "accounts" fixture). Every password is "password".
--
-- Ids, so a test can add rows of its own beside them:
--   accounts     101 - 114 (a test's own accounts get the next ids)
--   federations  1 Kernel, 2 Security (and 0 Default, from the
--                migration); a test's own get the next ids
--
-- Kernel holds -1001000000001 and -1001000000002, Security
-- -1001000000002 and -1001000000003, so Default holds the sample's
-- other groups: -1001000000004, -1001000000005 and -1001000000010.
--
--   fed_root        superadmin
--   kern_admin      admin of Kernel
--   sec_admin       admin of Security and of Default
--   idle_admin      admin of nothing
--   kern_member     member in Kernel
--   both_member     member in Kernel and Security
--   sec_member      member in Security
--   default_member  member in Default
--   loose_member    member in no federation

INSERT INTO web_users (id, username, display_name, password_hash, role, is_active,
	session_epoch, created_at, updated_at) VALUES
	(101, 'fed_root', 'Fed Root',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'superadmin', 1, 0, '2026-01-05 09:00:00', '2026-01-05 09:00:00'),
	(102, 'kern_admin', 'Kernel Admin',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2026-02-01 09:00:00', '2026-02-01 09:00:00'),
	(103, 'sec_admin', 'Security Admin',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2026-02-02 09:00:00', '2026-02-02 09:00:00'),
	(104, 'idle_admin', '',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'admin', 1, 0, '2026-02-03 09:00:00', '2026-02-03 09:00:00'),
	(110, 'kern_member', '',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'member', 1, 0, '2026-03-01 09:00:00', '2026-03-01 09:00:00'),
	(111, 'both_member', '',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'member', 1, 0, '2026-03-02 09:00:00', '2026-03-02 09:00:00'),
	(112, 'sec_member', '',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'member', 1, 0, '2026-03-03 09:00:00', '2026-03-03 09:00:00'),
	(113, 'default_member', '',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'member', 1, 0, '2026-03-04 09:00:00', '2026-03-04 09:00:00'),
	(114, 'loose_member', '',
	 '$argon2id$v=19$m=65536,t=2,p=1$Qct4Tg1+36Ma0aZdfudjQg$jWUvHvwLTZbcVTsBoWAMQWC+aUNFWqroYuulUWrw1lU',
	 'member', 1, 0, '2026-03-05 09:00:00', '2026-03-05 09:00:00');

INSERT INTO web_federations (id, name, created_by, created_at) VALUES
	(1, 'Kernel', 101, '2026-04-01 09:00:00'),
	(2, 'Security', 101, '2026-04-02 09:00:00');

INSERT INTO web_federation_groups (federation_id, group_id) VALUES
	(1, -1001000000001), (1, -1001000000002),
	(2, -1001000000002), (2, -1001000000003);

INSERT INTO web_federation_admins (federation_id, web_user_id) VALUES
	(1, 102), (2, 103), (0, 103);

INSERT INTO web_federation_members (federation_id, web_user_id) VALUES
	(1, 110), (1, 111), (2, 111), (2, 112), (0, 113);
