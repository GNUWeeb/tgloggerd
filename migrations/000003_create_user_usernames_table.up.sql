-- Normalized usernames for each user (td_api::usernames).
-- A user may hold several usernames of different kinds. Rows are
-- re-synced (deleted and re-inserted) when the username set changes, so
-- only a created_at timestamp is tracked.

CREATE TABLE user_usernames (
	-- Surrogate primary key.
	id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	-- Owning user.
	user_id    BIGINT          NOT NULL COMMENT 'FK to users.id.',
	-- The username text without the leading '@'.
	username   VARCHAR(32)     NOT NULL COMMENT 'Username without the leading @.',
	-- Which td_api::usernames list this username came from.
	kind       ENUM('active', 'disabled', 'editable', 'collectible') NOT NULL
	                           COMMENT 'Source list: active/disabled/editable/collectible.',
	-- Preserves order within the active/collectible lists (0-based).
	position   INT             NOT NULL DEFAULT 0 COMMENT 'Order within its list (0-based).',
	-- Row creation time.
	created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Row creation time.',

	PRIMARY KEY (id),
	UNIQUE KEY uq_user_username_kind (user_id, username, kind),
	KEY idx_user_usernames_user_id (user_id),
	KEY idx_user_usernames_username (username),
	CONSTRAINT fk_user_usernames_user
		FOREIGN KEY (user_id) REFERENCES users (id)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Normalized usernames per user (td_api::usernames).';
