-- Users table, modeled after the TDLib `user` object (td_api::user).
-- Stores the last-known state of each Telegram user seen by tgloggerd.
-- Volatile online/offline status is intentionally not stored here; per
-- attribute history (name, username, photo, ...) is tracked in dedicated
-- tables added later.

CREATE TABLE users (
	-- td_api::user.id (int53): Telegram user identifier.
	id                                 BIGINT          NOT NULL,

	-- Identity
	first_name                         VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'User first name.',
	last_name                          VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'User last name.',
	phone_number                       VARCHAR(32)     NOT NULL DEFAULT '' COMMENT 'User phone number, if visible.',

	-- td_api::UserType
	type                               ENUM('regular', 'deleted', 'bot', 'unknown')
	                                                   NOT NULL DEFAULT 'unknown' COMMENT 'Kind of user.',

	-- Profile photo; references the shared files table.
	profile_photo_file_id              BIGINT UNSIGNED NULL COMMENT 'FK to files.id for the current profile photo.',

	-- Appearance
	accent_color_id                    INT             NOT NULL DEFAULT 0 COMMENT 'td_api accent_color_id.',
	background_custom_emoji_id         BIGINT          NOT NULL DEFAULT 0 COMMENT 'Custom emoji id for the name background; 0 if none.',
	profile_accent_color_id            INT             NOT NULL DEFAULT -1 COMMENT 'Profile accent color id; -1 if none.',
	profile_background_custom_emoji_id BIGINT          NOT NULL DEFAULT 0 COMMENT 'Custom emoji id for the profile background; 0 if none.',

	-- td_api::emojiStatus
	emoji_status_custom_emoji_id       BIGINT          NULL COMMENT 'Custom emoji id shown as emoji status; NULL if none.',
	emoji_status_expiration_date       INT             NULL COMMENT 'Unix time when the emoji status expires; NULL if none.',

	-- td_api::verificationStatus
	is_verified                        TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User is verified by Telegram.',
	is_scam                            TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User is flagged as a scam.',
	is_fake                            TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User is flagged as fake.',

	is_premium                         TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User has Telegram Premium.',
	is_support                         TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User is a Telegram support account.',

	-- td_api::restrictionInfo
	restriction_reason                 VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'Reason the user is restricted; empty if none.',
	has_sensitive_content              TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User content is marked sensitive.',

	restricts_new_chats                TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'User may restrict new chats from non-contacts.',
	paid_message_star_count            BIGINT          NOT NULL DEFAULT 0 COMMENT 'Telegram Stars required to message the user.',

	-- Bookkeeping
	created_at                         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Row creation time.',
	updated_at                         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP
	                                                   ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last time the row was updated.',

	PRIMARY KEY (id),
	KEY idx_users_profile_photo_file_id (profile_photo_file_id),
	CONSTRAINT fk_users_profile_photo_file
		FOREIGN KEY (profile_photo_file_id) REFERENCES files (id)
		ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Last-known state of each Telegram user (td_api::user).';
