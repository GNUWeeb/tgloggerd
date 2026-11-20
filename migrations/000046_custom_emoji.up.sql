-- The stickers behind custom emoji (docs/design/message-types.md section
-- 4.11). Messages keep only a custom emoji's id -- in text entities,
-- reactions, forum topic icons, a sticker's custom_emoji_id -- and the web
-- needs the sticker to draw it. One row per custom emoji, filled on demand
-- from getCustomEmojiStickers; the sticker's file is a telegram_files row,
-- metadata only until someone views it, like any other file.
--
-- A new, empty table: nothing existing is rebuilt.
CREATE TABLE telegram_custom_emoji (
	custom_emoji_id BIGINT NOT NULL COMMENT 'Telegram custom emoji id.',
	emoji VARCHAR(16) NOT NULL DEFAULT '' COMMENT 'The emoji it stands for.',
	set_id BIGINT NULL DEFAULT NULL COMMENT 'Its sticker set; NULL if unknown.',
	format ENUM('webp','tgs','webm') NULL DEFAULT NULL COMMENT 'Sticker format; NULL if unknown.',
	needs_repainting TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Drawn in the text colour (like an emoji status).',
	file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id: the sticker; NULL if not recorded.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (custom_emoji_id),
	KEY idx_custom_emoji_file (file_id),
	CONSTRAINT fk_custom_emoji_file FOREIGN KEY (file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Stickers of custom emoji, filled on demand.';
