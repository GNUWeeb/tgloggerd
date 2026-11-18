-- The topics of forum groups (docs/design/message-types.md section 4.5).
-- A forum mixes many conversations in one chat; each group message names
-- its topic in topic_kind/topic_id (added by 000034), and this table says
-- what the topics are: their names, icons, who made them and when, and
-- whether they are closed, hidden or pinned, as last seen.
--
-- Keyed by the chat and TDLib's forum topic id. There is no foreign key to
-- telegram_groups: topic updates can arrive before the group's row is
-- written, and a topic must not be lost for that.
--
-- What only describes this account (unread counts, its notification
-- settings, its draft) is left out.
--
-- A new, empty table: nothing existing is rebuilt.
CREATE TABLE telegram_forum_topics (
	chat_id BIGINT NOT NULL COMMENT 'Telegram chat_id of the forum (telegram_groups.id).',
	topic_id INT NOT NULL COMMENT 'TDLib forum_topic_id; matches telegram_group_messages.topic_id.',
	name VARCHAR(255) NOT NULL DEFAULT '',
	is_name_implicit TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'The name was made up by the client, not set by anyone.',
	icon_color INT NOT NULL DEFAULT 0 COMMENT 'Icon colour, RGB.',
	icon_custom_emoji_id BIGINT NULL DEFAULT NULL COMMENT 'Custom emoji of the icon; NULL if none.',
	creation_date BIGINT NOT NULL DEFAULT 0 COMMENT 'Unix time it was created; 0 if unknown.',
	creator_user_id BIGINT NULL DEFAULT NULL COMMENT 'Who created it, if a user.',
	creator_chat_id BIGINT NULL DEFAULT NULL COMMENT 'Who created it, if a chat.',
	is_general TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'The forum''s General topic.',
	is_closed TINYINT(1) NOT NULL DEFAULT 0,
	is_hidden TINYINT(1) NOT NULL DEFAULT 0,
	is_pinned TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (chat_id, topic_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Topics of forum groups, as last seen.';
