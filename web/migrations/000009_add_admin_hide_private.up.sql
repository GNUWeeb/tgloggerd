-- An admin restriction (docs/design/web-accounts.md section 4,
-- docs/todo/web-admin-restrictions.md): a superadmin can hide private
-- chats and their messages from an admin. The request scope reads it
-- with the account's role on every request, so a change applies at once.
-- It means nothing for the other roles, and is cleared when an admin
-- becomes one of them.
--
-- Additive: one column with a default, so every existing admin keeps
-- seeing private chats until a superadmin hides them.
ALTER TABLE web_users
	ADD COLUMN hide_private TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Admin restriction: private chats hidden.' AFTER role;
