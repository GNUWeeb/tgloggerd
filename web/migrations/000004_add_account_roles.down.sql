-- Back to admin and viewer. Lossy: superadmin and admin both become
-- admin, so an admin gets every right again; member becomes viewer.
ALTER TABLE web_audit
	DROP KEY idx_web_audit_user_action,
	DROP KEY idx_web_audit_target,
	DROP COLUMN target_user_id;

ALTER TABLE web_users
	DROP KEY uq_web_users_email,
	DROP COLUMN created_by,
	DROP COLUMN must_change_password,
	DROP COLUMN email,
	DROP COLUMN display_name,
	MODIFY COLUMN role ENUM('admin', 'viewer', 'superadmin', 'member')
		NOT NULL DEFAULT 'viewer' COMMENT 'Access level.';

UPDATE web_users SET role = 'admin' WHERE role IN ('superadmin', 'admin');
UPDATE web_users SET role = 'viewer' WHERE role = 'member';

ALTER TABLE web_users
	MODIFY COLUMN role ENUM('admin', 'viewer')
		NOT NULL DEFAULT 'viewer' COMMENT 'Access level.';
