-- The member count cache of telegram_groups, as daemon migration
-- 000044 adds it (member_count, member_count_at, can_get_members),
-- for the tests of the web's member counts until that migration is in
-- migrations/. Each column is added only when missing, so this file
-- does nothing once the migration has added them, and can then go.
SET @tgd_sql = (SELECT IF(COUNT(*) = 0,
	'ALTER TABLE telegram_groups ADD COLUMN member_count INT UNSIGNED NULL, ADD COLUMN member_count_at DATETIME NULL, ADD COLUMN can_get_members TINYINT(1) NULL',
	'DO 0')
	FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE()
	AND TABLE_NAME = 'telegram_groups' AND COLUMN_NAME = 'member_count');
PREPARE tgd_stmt FROM @tgd_sql;
EXECUTE tgd_stmt;
DEALLOCATE PREPARE tgd_stmt;
