ALTER TABLE telegram_groups
	DROP INDEX ftx_groups_description;

ALTER TABLE telegram_groups
	DROP COLUMN can_get_members,
	DROP COLUMN member_count_at,
	DROP COLUMN member_count;

ALTER TABLE telegram_groups
	ADD FULLTEXT INDEX ftx_groups_description (description);
