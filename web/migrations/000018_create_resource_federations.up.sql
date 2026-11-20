-- The federation of each forwarding configuration (docs/design/
-- web-accounts.md section 6): chosen when it is created, among the
-- federations holding its chat, and deciding which admins may give it
-- to another account.
--
-- Additive: a new table only, and no row is written here. A
-- configuration without a row is in Default, which is where every
-- configuration made before this table was; deleting a federation
-- deletes its rows, so its configurations fall back to Default too.
--
-- The configurations live in the logger's schema, so, as for
-- web_resource_relations, there is no foreign key to them; the web
-- service writes the row when it creates one and drops it when it
-- deletes one. resource_type takes the values of
-- web_resource_relations (t2d_webhook, d2t_route).
CREATE TABLE web_resource_federations (
	resource_type VARCHAR(32)     NOT NULL COMMENT 'As in web_resource_relations.',
	resource_id   BIGINT          NOT NULL COMMENT 'The resource''s id in its table.',
	federation_id BIGINT UNSIGNED NOT NULL COMMENT 'web_federations.id, never 0 (Default has no row).',
	created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (resource_type, resource_id),
	KEY idx_web_resource_federations_fed (federation_id),
	CONSTRAINT fk_web_resource_federations_fed FOREIGN KEY (federation_id)
		REFERENCES web_federations (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Federation of each forwarding configuration; none is Default.';
