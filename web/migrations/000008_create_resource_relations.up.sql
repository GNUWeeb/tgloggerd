-- Who owns what (docs/design/web-accounts.md section 6): one generic
-- table of relations between an account and a resource, so further
-- relations (sharing as editor or viewer) need no new table and no
-- change to the resources' own tables.
--
-- Additive: a new table only. No row is written here. A resource with
-- no owner row counts as owned by the superadmins; the webapi command
-- `assign-owner <username>` gives every such resource to one account,
-- once, at deployment (web/docs/accounts.md, Ownership).
--
-- Most resources live in the logger's schema, so there is no foreign
-- key to them; the web service writes a relation when it creates a
-- resource and drops it when it deletes one. resource_type names the
-- table resource_id points into:
--
--   mcp_group      telegram_public_groups.group_id (negative chat id)
--   mcp_token      web_mcp_tokens.id
--   telegram_bot   telegram_bots.id
--   discord_bot    the Discord bots table (discord-bots-per-user)
--   t2d_webhook    telegram_discord_webhooks.id
--   d2t_route      discord_telegram_routes.id
CREATE TABLE web_resource_relations (
	resource_type VARCHAR(32)     NOT NULL COMMENT 'Which table resource_id is in (see above).',
	resource_id   BIGINT          NOT NULL COMMENT 'The resource''s id there; signed for group ids.',
	relation      VARCHAR(16)     NOT NULL COMMENT '"owner" today.',
	user_id       BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id holding the relation.',
	created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by    BIGINT UNSIGNED NULL DEFAULT NULL
	                              COMMENT 'web_users.id that made the change; NULL for the assign-owner command.',
	-- 1 for an owner row, NULL for any other relation: the unique key
	-- below then allows one owner per resource and leaves the other
	-- relations (NULLs never clash) to the primary key.
	sole_owner    TINYINT AS (IF(relation = 'owner', 1, NULL)) STORED,

	PRIMARY KEY (resource_type, resource_id, relation, user_id),
	UNIQUE KEY uq_web_resource_relations_owner (resource_type, resource_id, sole_owner),
	-- What one account holds, by relation and type.
	KEY idx_web_resource_relations_user (user_id, relation, resource_type),
	-- Accounts are disabled, not deleted, so a resource never loses its
	-- owner unnoticed: deleting an account that holds one is refused.
	CONSTRAINT fk_web_resource_relations_user FOREIGN KEY (user_id)
		REFERENCES web_users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Relations between accounts and resources; owner today.';
