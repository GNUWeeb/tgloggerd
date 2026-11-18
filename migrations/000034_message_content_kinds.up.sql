-- Content kinds, the exact TDLib class and structured detail for messages
-- (docs/design/message-types.md sections 4.1 and 4.2), and every other
-- column the message types design adds to the two message tables (sections
-- 4.5 to 4.7), so that those tables are rebuilt once.
--
-- content_type keeps its coarse kinds and gains one per kind a reader sees
-- differently, appended at the end so the existing values keep their
-- internal numbers. 'unknown' from now on means a TDLib class newer than the
-- daemon's code.
--
-- tdlib_type: the exact TDLib class of the content, e.g. messageVideoNote.
-- content_extra: the class's own fields that have no column, as a curated
-- JSON object per class, or TDLib's own JSON under the key "tdlib" for a
-- class without a mapping. The edit tables get both, since an edit can
-- change them.
--
-- topic_kind, topic_id (group messages): td_api::message.topic_id.
-- is_pinned, meta: the pinned state and the message-level fields without a
-- column of their own. view_count, forward_count, reply_count, reactions:
-- the latest interaction totals. All are NULL (is_pinned 0) until the
-- daemon fills them in.
--
-- telegram_files.file_type gains the kinds of files that messages carry
-- beyond the eight it has: round video notes, and thumbnails.
--
-- How the message tables are changed. They have a FULLTEXT index on text,
-- and with it InnoDB can neither add a column nor change the ENUM in place:
-- only a copy of the table works, which maintains the full-text index row
-- by row and takes hours on a large archive. Without the index, appending
-- ENUM members is a metadata change and adding columns is an online
-- rebuild that takes minutes; building the index again afterwards takes
-- minutes too. So each table drops its FULLTEXT index, is altered, and gets
-- the index back. Full-text search fails while the index is missing, and
-- writes to the table wait while it is rebuilt. If the migration stops in
-- between, the index may be missing: re-run the ADD FULLTEXT statement
-- below by hand before forcing the version.
--
-- The edit tables and telegram_files have no FULLTEXT index and change
-- instantly; ALGORITHM=INSTANT makes a server that cannot do that fail the
-- statement instead of copying the table.

ALTER TABLE telegram_group_messages
	DROP INDEX ftx_group_messages_text, ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE telegram_group_messages
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown',
		'video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported')
		NOT NULL DEFAULT 'unknown' COMMENT 'Coarse message content category.',
	ADD COLUMN tdlib_type VARCHAR(64) NULL DEFAULT NULL
		COMMENT 'Exact TDLib content class, e.g. messageVideoNote; NULL if not recorded.'
		AFTER service_type,
	ADD COLUMN content_extra JSON NULL DEFAULT NULL
		COMMENT 'Per-class content detail (docs/design/message-types.md); NULL if none.'
		AFTER tdlib_type,
	ADD COLUMN topic_kind ENUM('forum','thread','direct','saved') NULL DEFAULT NULL
		COMMENT 'Kind of td_api::message.topic_id; NULL if none or not recorded.'
		AFTER reply_to_id,
	ADD COLUMN topic_id BIGINT NULL DEFAULT NULL
		COMMENT 'Forum topic, thread or topic id within the chat; NULL if none.'
		AFTER topic_kind,
	ADD COLUMN is_pinned TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Message is pinned in the chat, as last observed.'
		AFTER topic_id,
	ADD COLUMN meta JSON NULL DEFAULT NULL
		COMMENT 'Message-level detail without a column (via bot, quote, markup, ...); NULL if none.'
		AFTER is_pinned,
	ADD COLUMN view_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Latest view count; NULL if never observed.'
		AFTER meta,
	ADD COLUMN forward_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Latest forward count; NULL if never observed.'
		AFTER view_count,
	ADD COLUMN reply_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Latest reply count; NULL if never observed.'
		AFTER forward_count,
	ADD COLUMN reactions JSON NULL DEFAULT NULL
		COMMENT 'Latest reaction totals; NULL if never observed.'
		AFTER reply_count,
	ADD KEY idx_group_messages_chat_topic (chat_id, topic_kind, topic_id),
	ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE telegram_group_messages
	ADD FULLTEXT INDEX ftx_group_messages_text (text);

ALTER TABLE telegram_private_messages
	DROP INDEX ftx_private_messages_text, ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE telegram_private_messages
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown',
		'video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported')
		NOT NULL DEFAULT 'unknown' COMMENT 'Coarse message content category.',
	ADD COLUMN tdlib_type VARCHAR(64) NULL DEFAULT NULL
		COMMENT 'Exact TDLib content class, e.g. messageVideoNote; NULL if not recorded.'
		AFTER service_type,
	ADD COLUMN content_extra JSON NULL DEFAULT NULL
		COMMENT 'Per-class content detail (docs/design/message-types.md); NULL if none.'
		AFTER tdlib_type,
	ADD COLUMN is_pinned TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Message is pinned in the chat, as last observed.'
		AFTER reply_to_id,
	ADD COLUMN meta JSON NULL DEFAULT NULL
		COMMENT 'Message-level detail without a column (via bot, quote, markup, ...); NULL if none.'
		AFTER is_pinned,
	ADD COLUMN view_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Latest view count; NULL if never observed.'
		AFTER meta,
	ADD COLUMN forward_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Latest forward count; NULL if never observed.'
		AFTER view_count,
	ADD COLUMN reply_count INT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Latest reply count; NULL if never observed.'
		AFTER forward_count,
	ADD COLUMN reactions JSON NULL DEFAULT NULL
		COMMENT 'Latest reaction totals; NULL if never observed.'
		AFTER reply_count,
	ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE telegram_private_messages
	ADD FULLTEXT INDEX ftx_private_messages_text (text);

ALTER TABLE telegram_group_message_edits
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown',
		'video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported')
		NOT NULL DEFAULT 'unknown' COMMENT 'Content type before the edit.',
	ADD COLUMN tdlib_type VARCHAR(64) NULL DEFAULT NULL
		COMMENT 'Exact TDLib content class before the edit; NULL if not recorded.'
		AFTER entities,
	ADD COLUMN content_extra JSON NULL DEFAULT NULL
		COMMENT 'Per-class content detail before the edit; NULL if none.'
		AFTER tdlib_type,
	ALGORITHM=INSTANT;

ALTER TABLE telegram_private_message_edits
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown',
		'video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported')
		NOT NULL DEFAULT 'unknown' COMMENT 'Content type before the edit.',
	ADD COLUMN tdlib_type VARCHAR(64) NULL DEFAULT NULL
		COMMENT 'Exact TDLib content class before the edit; NULL if not recorded.'
		AFTER entities,
	ADD COLUMN content_extra JSON NULL DEFAULT NULL
		COMMENT 'Per-class content detail before the edit; NULL if none.'
		AFTER tdlib_type,
	ALGORITHM=INSTANT;

ALTER TABLE telegram_files
	MODIFY COLUMN file_type ENUM('photo','video','document','audio','voice','sticker','animation','unknown',
		'video_note','thumbnail')
		NOT NULL DEFAULT 'unknown',
	ALGORITHM=INSTANT;
