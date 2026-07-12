-- Bot-specific attributes for users whose type is 'bot'
-- (td_api::userTypeBot). One-to-one with the users table.

CREATE TABLE user_bot_info (
	-- Owning bot user; also the primary key (one row per bot user).
	user_id                         BIGINT       NOT NULL COMMENT 'FK to users.id; the bot user.',

	can_be_edited                   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'The current account can edit this bot.',
	can_join_groups                 TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot can be added to groups.',
	can_read_all_group_messages     TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot receives all group messages, not only commands.',
	has_main_web_app                TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot has a main Web App.',
	has_topics                      TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot supports topics.',
	allows_users_to_create_topics   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot lets users create topics.',
	can_manage_bots                 TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot can manage other bots.',
	is_inline                       TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot supports inline queries.',
	inline_query_placeholder        VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Placeholder shown in the inline query field.',
	supports_guest_queries          TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot supports queries from guest users.',
	is_guard                        TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot is a guard bot.',
	need_location                   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot requests the user location for inline queries.',
	can_connect_to_business         TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot can be connected to a Telegram Business account.',
	can_be_added_to_attachment_menu TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'Bot can be added to the attachment menu.',
	active_user_count               INT          NOT NULL DEFAULT 0 COMMENT 'Approximate number of active users of the bot.',

	-- Bookkeeping
	created_at                      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Row creation time.',
	updated_at                      TIMESTAMP    NULL DEFAULT NULL
	                                             ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last time the row was updated; NULL until first update.',

	PRIMARY KEY (user_id),
	CONSTRAINT fk_user_bot_info_user
		FOREIGN KEY (user_id) REFERENCES users (id)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Bot-specific attributes for users of type bot (td_api::userTypeBot).';
