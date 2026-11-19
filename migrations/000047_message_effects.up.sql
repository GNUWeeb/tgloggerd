-- The effects a message can carry (meta.effect_id; docs/design/
-- message-types.md section 4.12). A message keeps only the effect's id;
-- the web needs its emoji and animations to show it. An effect is not a
-- custom emoji: getMessageEffect answers a messageEffect of up to four
-- stickers (a static icon, and either the select and effect animations
-- of an emoji reaction or a premium sticker and its premium animation),
-- so it gets a table of its own. One row per effect, filled on demand;
-- each file is a telegram_files row, metadata only until someone views
-- it, like any other file.
--
-- A new, empty table: nothing existing is rebuilt.
CREATE TABLE telegram_message_effects (
	effect_id BIGINT NOT NULL COMMENT 'Telegram message effect id.',
	emoji VARCHAR(16) NOT NULL DEFAULT ''
		COMMENT 'The emoji shown when no icon is available.',
	is_premium TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Only Telegram Premium accounts can send it.',
	effect_type ENUM('emoji_reaction','premium_sticker') NULL DEFAULT NULL
		COMMENT 'messageEffectType; NULL if unknown.',
	static_icon_file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id: the static icon (WEBP); NULL if none.',
	select_animation_file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id: an emoji reaction''s select animation (TGS).',
	effect_animation_file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id: the effect animation (TGS), either type.',
	sticker_file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id: a premium sticker effect''s sticker.',
	sticker_format ENUM('webp','tgs','webm') NULL DEFAULT NULL
		COMMENT 'Format of sticker_file_id; NULL if none or unknown.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (effect_id),
	KEY idx_message_effect_static_icon (static_icon_file_id),
	KEY idx_message_effect_select_animation (select_animation_file_id),
	KEY idx_message_effect_effect_animation (effect_animation_file_id),
	KEY idx_message_effect_sticker (sticker_file_id),
	CONSTRAINT fk_message_effect_static_icon FOREIGN KEY (static_icon_file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE,
	CONSTRAINT fk_message_effect_select_animation FOREIGN KEY (select_animation_file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE,
	CONSTRAINT fk_message_effect_effect_animation FOREIGN KEY (effect_animation_file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE,
	CONSTRAINT fk_message_effect_sticker FOREIGN KEY (sticker_file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Message effects, filled on demand.';
