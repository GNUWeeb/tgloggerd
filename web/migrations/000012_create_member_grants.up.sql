-- Member grants (docs/design/web-accounts.md section 5,
-- docs/todo/web-member-access.md): a member sees only the groups
-- granted to it, a subset of the groups of the federations it is filed
-- in, and may be given the abilities forward and fetch_files. The
-- request scope reads them with the account's role on every request,
-- so a change applies at once.
--
-- Additive: new tables only; no member has any grant until an admin of
-- its federation or a superadmin gives it one.
--
-- A grant hangs off the member's filing in the federation
-- (web_federation_members), so it goes when the member leaves the
-- federation, when the federation is deleted, and when the account
-- stops being a member (its filings go then). group_id is the logger's
-- (negative) chat id, in another schema, so there is no foreign key to
-- it; the web service drops a grant when its group leaves the
-- federation, and the scope reads only grants whose group is still in
-- the federation (for Default: in no other federation).
CREATE TABLE web_member_grant_groups (
	web_user_id   BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id, a member.',
	federation_id BIGINT UNSIGNED NOT NULL COMMENT 'web_federations.id the grant is made in.',
	group_id      BIGINT          NOT NULL COMMENT 'telegram_groups.id (negative chat id).',
	created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (web_user_id, federation_id, group_id),
	KEY idx_web_member_grant_groups_filing (federation_id, web_user_id),
	KEY idx_web_member_grant_groups_group (group_id),
	CONSTRAINT fk_web_member_grant_groups_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE,
	CONSTRAINT fk_web_member_grant_groups_fed FOREIGN KEY (federation_id)
		REFERENCES web_federations (id) ON DELETE CASCADE,
	CONSTRAINT fk_web_member_grant_groups_filing FOREIGN KEY (federation_id, web_user_id)
		REFERENCES web_federation_members (federation_id, web_user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Groups granted to a member within a federation (web-member-access).';

-- What a member may do beyond seeing its groups. Meaningless for the
-- other roles: the web service drops them when the account stops
-- being a member.
CREATE TABLE web_member_abilities (
	web_user_id BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id, a member.',
	ability     ENUM('forward', 'fetch_files') NOT NULL
	                            COMMENT 'forward: set up forwarding; fetch_files: fetch unstored files.',
	created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (web_user_id, ability),
	CONSTRAINT fk_web_member_abilities_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Abilities given to a member (web-member-access).';
