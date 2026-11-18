-- The files a message carries beyond its main one
-- (docs/design/message-types.md section 4.3): a video's cover, the video of
-- a live photo, each item of paid media, a game's or an invoice's picture,
-- the new photo of a chat photo change. The main file stays in the message
-- row's file_id; this table holds the others, one row per file, by role and
-- position within the role.
--
-- One row per (message, role, position). Exactly one of group_message_id
-- and private_message_id is set, as in telegram_link_previews, and for the
-- same reason not as a CHECK: MySQL refuses one over columns that a foreign
-- key with ON DELETE CASCADE uses. file_id is NULL until the file is
-- linked, and again if its telegram_files row is deleted.
--
-- A new, empty table: nothing existing is rebuilt.
CREATE TABLE telegram_message_files (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	group_message_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_group_messages.id.',
	private_message_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_private_messages.id.',
	role ENUM('cover','live_video','paid','preview','poll','game','invoice',
		  'chat_photo','thumbnail') NOT NULL
		COMMENT 'What the file is to the message.',
	position SMALLINT UNSIGNED NOT NULL DEFAULT 0
		COMMENT 'Order within the role, from 0 (paid media items).',
	file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id; NULL until linked.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_message_files_group (group_message_id, role, position),
	UNIQUE KEY uq_message_files_private (private_message_id, role, position),
	KEY idx_message_files_file (file_id),
	CONSTRAINT fk_message_files_group_msg FOREIGN KEY (group_message_id)
		REFERENCES telegram_group_messages (id)
		ON DELETE CASCADE ON UPDATE CASCADE,
	CONSTRAINT fk_message_files_private_msg FOREIGN KEY (private_message_id)
		REFERENCES telegram_private_messages (id)
		ON DELETE CASCADE ON UPDATE CASCADE,
	CONSTRAINT fk_message_files_file FOREIGN KEY (file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Files of a message beyond its main one (file_id).';
