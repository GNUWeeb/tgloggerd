-- Telegram communities (docs/design/message-types.md section 4.13). A
-- community groups supergroups, channels and bot chats; the service
-- lines chat_join_from_community and chat_added_to_community keep only
-- its id, which is not a chat id. TDLib has no getCommunity: a
-- community's name and photo arrive in updateCommunity, and its chats
-- and big photo in its full info (getCommunityFullInfo,
-- updateCommunityFullInfo).
--
-- New, empty tables: nothing existing is rebuilt.
CREATE TABLE telegram_communities (
	id BIGINT NOT NULL COMMENT 'TDLib community id.',
	name VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Community name.',
	have_access TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'TDLib have_access: 0 means only this row is known.',
	photo_file_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'FK to telegram_files.id: the big (640x640) photo, else the small one; NULL if none.',
	administrator_count INT NULL DEFAULT NULL
		COMMENT 'From the full info; NULL unless the account administers it.',
	date BIGINT NULL DEFAULT NULL
		COMMENT 'TDLib date (Unix seconds): when joined, else when created.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_community_photo (photo_file_id),
	CONSTRAINT fk_community_photo FOREIGN KEY (photo_file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Telegram communities, from updateCommunity and their full info.';

-- The chats of a community, as its last full info listed them. No
-- foreign key to the chat tables: a community's chats need not be
-- stored.
CREATE TABLE telegram_community_chats (
	community_id BIGINT NOT NULL COMMENT 'FK to telegram_communities.id.',
	chat_id BIGINT NOT NULL COMMENT 'TDLib chat id of a chat in it.',
	is_hidden TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Hidden from the community''s chat list (shown to administrators only).',
	can_view_history TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'The chat''s message history can be viewed.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (community_id, chat_id),
	KEY idx_community_chat_chat (chat_id),
	CONSTRAINT fk_community_chat_community FOREIGN KEY (community_id)
		REFERENCES telegram_communities (id)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Chats of each community, replaced when its full info arrives.';
