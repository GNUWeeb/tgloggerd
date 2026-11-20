-- Hidden member lists (the group members work): a superadmin can hide
-- a channel's member list, which the web then lists for superadmins
-- only; everyone else who sees the channel gets its member count and
-- "hidden" in place of the list.
--
-- Additive: a new table only. group_id is the logger's (negative) chat
-- id, in another schema, so there is no foreign key to it. A list stays
-- hidden when the superadmin who hid it loses its account.
CREATE TABLE web_hidden_member_lists (
	group_id   BIGINT          NOT NULL COMMENT 'telegram_groups.id (negative chat id), a channel.',
	hidden_by  BIGINT UNSIGNED NULL     COMMENT 'web_users.id of the superadmin who hid it.',
	created_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (group_id),
	CONSTRAINT fk_web_hidden_member_lists_user FOREIGN KEY (hidden_by)
		REFERENCES web_users (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Channels whose member list only superadmins see (group members).';
