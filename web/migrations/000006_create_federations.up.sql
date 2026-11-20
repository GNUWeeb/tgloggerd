-- Federations (docs/design/web-accounts.md section 3): named sets of
-- Telegram groups, defined by superadmins, with admins and members
-- assigned. They limit what an admin manages, not what it sees.
--
-- Additive: new tables only, and rows that keep today's rights. Until
-- now the Default federation was the only one, every admin
-- administered it and every member was in it, so an admin managed
-- every member; the rows inserted below keep exactly that.
--
-- The Default federation is a stored row, id 0 (the id the API uses
-- for it), so admins and members can be assigned to it
-- (decisions/2026-11-18-web-accounts-details.md). Its groups are
-- derived, never stored: a group in no other federation is in Default,
-- including every group joined later. The ids are not AUTO_INCREMENT:
-- MySQL would renumber the row 0 whenever rows are copied into the
-- table (INSERT ... SELECT, a dump loaded without
-- NO_AUTO_VALUE_ON_ZERO, an ALTER that copies the table), so the web
-- service picks the next id itself.
CREATE TABLE web_federations (
	id         BIGINT UNSIGNED NOT NULL COMMENT '0 is Default; others MAX(id) + 1 when made.',
	name       VARCHAR(64)     NOT NULL COMMENT 'Shown name; unique.',
	created_by BIGINT UNSIGNED NULL DEFAULT NULL
	                           COMMENT 'web_users.id of the superadmin that made it; NULL for Default.',
	created_at DATETIME        NULL DEFAULT CURRENT_TIMESTAMP
	                           COMMENT 'When it was made; NULL for Default.',

	PRIMARY KEY (id),
	UNIQUE KEY uq_web_federations_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Federations of groups; Default (id 0) holds the groups in no other.';

-- The groups of each federation but Default. A group may be in
-- several. The group lives in the logger's schema, so there is no
-- foreign key to it.
CREATE TABLE web_federation_groups (
	federation_id BIGINT UNSIGNED NOT NULL COMMENT 'web_federations.id, never 0.',
	group_id      BIGINT          NOT NULL COMMENT 'telegram_groups.id (negative chat id).',
	added_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (federation_id, group_id),
	KEY idx_web_federation_groups_group (group_id),
	CONSTRAINT fk_web_federation_groups_fed FOREIGN KEY (federation_id)
		REFERENCES web_federations (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Groups of the federations other than Default.';

-- Which admins administer which federations. Superadmins administer
-- every federation without rows here.
CREATE TABLE web_federation_admins (
	federation_id BIGINT UNSIGNED NOT NULL COMMENT 'web_federations.id.',
	web_user_id   BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id of an admin.',
	added_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (federation_id, web_user_id),
	KEY idx_web_federation_admins_user (web_user_id),
	CONSTRAINT fk_web_federation_admins_fed FOREIGN KEY (federation_id)
		REFERENCES web_federations (id) ON DELETE CASCADE,
	CONSTRAINT fk_web_federation_admins_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Admins of each federation.';

-- The federations each member is filed in; a member may be in
-- several. The groups granted to it inside them are web-member-access's.
CREATE TABLE web_federation_members (
	federation_id BIGINT UNSIGNED NOT NULL COMMENT 'web_federations.id.',
	web_user_id   BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id of a member.',
	added_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (federation_id, web_user_id),
	KEY idx_web_federation_members_user (web_user_id),
	CONSTRAINT fk_web_federation_members_fed FOREIGN KEY (federation_id)
		REFERENCES web_federations (id) ON DELETE CASCADE,
	CONSTRAINT fk_web_federation_members_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Members filed in each federation.';

INSERT INTO web_federations (id, name, created_by, created_at)
	VALUES (0, 'Default', NULL, NULL);

-- Today's admins administer Default and today's members are filed in
-- it, disabled ones included, so nothing changes for them.
INSERT INTO web_federation_admins (federation_id, web_user_id)
	SELECT 0, id FROM web_users WHERE role = 'admin';
INSERT INTO web_federation_members (federation_id, web_user_id)
	SELECT 0, id FROM web_users WHERE role = 'member';
