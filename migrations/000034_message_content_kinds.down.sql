-- Back to the schema of 000033. Rows of the new kinds become 'unknown',
-- which is what they were stored as before; the new columns are dropped
-- with what they hold. The message tables lose their FULLTEXT index for the
-- duration, as in the up migration and for the same reason.
UPDATE telegram_group_messages SET content_type = 'unknown', updated_at = updated_at
	WHERE content_type IN ('video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported');
UPDATE telegram_private_messages SET content_type = 'unknown', updated_at = updated_at
	WHERE content_type IN ('video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported');
UPDATE telegram_group_message_edits SET content_type = 'unknown'
	WHERE content_type IN ('video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported');
UPDATE telegram_private_message_edits SET content_type = 'unknown'
	WHERE content_type IN ('video_note','location','venue','contact','dice','game','poll','checklist','story','paid_media','invoice','giveaway','call','rich','widget','expired','unsupported');
UPDATE telegram_files SET file_type = 'unknown', updated_at = updated_at
	WHERE file_type IN ('video_note','thumbnail');

ALTER TABLE telegram_group_messages
	DROP INDEX ftx_group_messages_text;

ALTER TABLE telegram_group_messages
	DROP KEY idx_group_messages_chat_topic,
	DROP COLUMN reactions,
	DROP COLUMN reply_count,
	DROP COLUMN forward_count,
	DROP COLUMN view_count,
	DROP COLUMN meta,
	DROP COLUMN is_pinned,
	DROP COLUMN topic_id,
	DROP COLUMN topic_kind,
	DROP COLUMN content_extra,
	DROP COLUMN tdlib_type,
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown')
		NOT NULL DEFAULT 'unknown' COMMENT 'Coarse message content category.';

ALTER TABLE telegram_group_messages
	ADD FULLTEXT INDEX ftx_group_messages_text (text);

ALTER TABLE telegram_private_messages
	DROP INDEX ftx_private_messages_text;

ALTER TABLE telegram_private_messages
	DROP COLUMN reactions,
	DROP COLUMN reply_count,
	DROP COLUMN forward_count,
	DROP COLUMN view_count,
	DROP COLUMN meta,
	DROP COLUMN is_pinned,
	DROP COLUMN content_extra,
	DROP COLUMN tdlib_type,
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown')
		NOT NULL DEFAULT 'unknown' COMMENT 'Coarse message content category.';

ALTER TABLE telegram_private_messages
	ADD FULLTEXT INDEX ftx_private_messages_text (text);

ALTER TABLE telegram_group_message_edits
	DROP COLUMN content_extra,
	DROP COLUMN tdlib_type,
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown')
		NOT NULL DEFAULT 'unknown' COMMENT 'Content type before the edit.';

ALTER TABLE telegram_private_message_edits
	DROP COLUMN content_extra,
	DROP COLUMN tdlib_type,
	MODIFY COLUMN content_type ENUM('text','photo','video','document','audio','voice','sticker','animation','service','unknown')
		NOT NULL DEFAULT 'unknown' COMMENT 'Content type before the edit.';

ALTER TABLE telegram_files
	MODIFY COLUMN file_type ENUM('photo','video','document','audio','voice','sticker','animation','unknown')
		NOT NULL DEFAULT 'unknown';
