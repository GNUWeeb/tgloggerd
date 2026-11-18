-- An admin restriction (docs/design/web-accounts.md section 4,
-- docs/todo/web-admin-restrictions.md): a superadmin can hide groups
-- from an admin. The request scope reads them with the account's role on
-- every request, so a change applies at once; they mean nothing for the
-- other roles and go when an admin becomes one of them.
--
-- Additive: a new table only. group_id is the logger's (negative) chat
-- id, in another schema, so there is no foreign key to it.
CREATE TABLE web_admin_hidden_groups (
	web_user_id BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id, an admin.',
	group_id    BIGINT          NOT NULL COMMENT 'telegram_groups.id (negative chat id).',
	created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (web_user_id, group_id),
	CONSTRAINT fk_web_admin_hidden_groups_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Groups hidden from an admin (web-admin-restrictions).';
