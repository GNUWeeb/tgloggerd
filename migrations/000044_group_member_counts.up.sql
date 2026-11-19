-- A group's member count as Telegram last reported it, when, and whether
-- the account may list its members (docs/design/group-members.md). The
-- daemon writes them whenever Telegram sends a group's full info or a
-- supergroup update, and when the web asks for a refresh, so the cache
-- fills in as groups are seen and is what the web shows when Telegram
-- cannot be asked.
--
-- member_count: NULL until known. member_count_at: when member_count was
-- last written. can_get_members: whether the account may list the
-- members (supergroup full info; always for a basic group it is in);
-- NULL until known.
--
-- telegram_groups has a FULLTEXT index on description, with which InnoDB
-- can only copy the table to add a column, maintaining the index row by
-- row. As for the message tables (decision
-- 2026-11-18-msg-message-table-ddl), the index is dropped, the columns
-- are added in place, and the index is built again. If the migration
-- stops in between, re-run the ADD FULLTEXT statement below by hand
-- before forcing the version.
ALTER TABLE telegram_groups
	DROP INDEX ftx_groups_description, ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE telegram_groups
	ADD COLUMN member_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Members, as Telegram last reported; NULL if unknown.'
		AFTER msg_count,
	ADD COLUMN member_count_at DATETIME NULL DEFAULT NULL
		COMMENT 'When member_count was last written.'
		AFTER member_count,
	ADD COLUMN can_get_members TINYINT(1) NULL DEFAULT NULL
		COMMENT 'The account may list the members; NULL if unknown.'
		AFTER member_count_at,
	ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE telegram_groups
	ADD FULLTEXT INDEX ftx_groups_description (description);
