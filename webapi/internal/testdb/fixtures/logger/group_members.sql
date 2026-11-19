-- Dummy member counts for the group member tests (the daemon's cache
-- of Telegram's counts in telegram_groups), loaded after "sample".
-- Invented for tests.
--
-- GNU/Weeb      1234 members, stored now (fresh)
-- Linux Kernel  500 members, stored long ago (stale)
-- Rust          none stored yet
-- Assembly      880 members, stored now; a channel whose members the
--               account may not list

UPDATE telegram_groups SET member_count = 1234, member_count_at = NOW(), can_get_members = 1,
	updated_at = updated_at
	WHERE id = -1001000000001;
UPDATE telegram_groups SET member_count = 500, member_count_at = '2026-01-01 00:00:00',
	can_get_members = 1, updated_at = updated_at WHERE id = -1001000000002;
UPDATE telegram_groups SET member_count = 880, member_count_at = NOW(), can_get_members = 0,
	updated_at = updated_at
	WHERE id = -1001000000005;
