-- Several web accounts with three roles (docs/design/web-accounts.md
-- section 2): superadmin, admin and member.
--
-- Additive: the existing rows stay, with their passwords and session
-- epochs. Today's `admin` had every right, so it becomes superadmin;
-- `viewer` becomes member. The column is widened first, so the rows can
-- be renamed in place, then narrowed to the three new roles.
--
-- Session cookies issued before this migration still carry the old
-- role name; both web services read the role from this table (the Go
-- service) or treat "admin" like "superadmin" (the Drogon pages), so
-- they stay valid.
ALTER TABLE web_users
	MODIFY COLUMN role ENUM('admin', 'viewer', 'superadmin', 'member')
		NOT NULL DEFAULT 'viewer' COMMENT 'Access level.';

UPDATE web_users SET role = 'superadmin' WHERE role = 'admin';
UPDATE web_users SET role = 'member' WHERE role = 'viewer';

ALTER TABLE web_users
	MODIFY COLUMN role ENUM('superadmin', 'admin', 'member')
		NOT NULL DEFAULT 'member'
		COMMENT 'superadmin, admin or member (web-accounts.md section 2).',
	-- is_active stays the disabled flag: accounts are disabled, never
	-- deleted, so audit rows and ownership keep pointing at them.
	ADD COLUMN display_name VARCHAR(128) NOT NULL DEFAULT ''
		COMMENT 'Name shown in the dashboard; may be empty.'
		AFTER username,
	-- The address an account is created with. Verification and the
	-- links sent to it come with the email work; until then it is only
	-- recorded. NULL for the accounts made before it existed.
	ADD COLUMN email VARCHAR(254) NULL DEFAULT NULL
		COMMENT 'Email address; NULL until one is given.'
		AFTER display_name,
	ADD COLUMN must_change_password TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'The password was set for the account; it must choose its own at the next sign-in.'
		AFTER session_epoch,
	-- No foreign key: the creator is disabled at most, never deleted,
	-- and the accounts made before this column have none.
	ADD COLUMN created_by BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'web_users.id of the account that created this one; NULL if none.'
		AFTER must_change_password,
	ADD UNIQUE KEY uq_web_users_email (email);

-- Account changes are audited with who did it (web_user_id) and the
-- account changed (target_user_id).
ALTER TABLE web_audit
	ADD COLUMN target_user_id BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'web_users.id of the account an account change applied to.'
		AFTER web_user_id,
	ADD KEY idx_web_audit_target (target_user_id),
	-- An account's last sign-in (its newest login_ok), without reading
	-- every media row the account left.
	ADD KEY idx_web_audit_user_action (web_user_id, action, created_at);
