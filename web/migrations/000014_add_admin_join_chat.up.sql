-- The last admin restriction (docs/design/web-accounts.md section 4):
-- a superadmin can grant an admin Join chat, which acts on the shared
-- Telegram account. The request scope reads it with the account's role
-- on every request, so a change applies at once. It means nothing for
-- the other roles (superadmins always may join chats, members never),
-- and is cleared when an admin becomes one of them.
--
-- Additive: one column that defaults to off, so no existing admin gains
-- Join chat until a superadmin grants it.
ALTER TABLE web_users
	ADD COLUMN join_chat TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Admin grant: may join chats (Join chat page).' AFTER hide_private;
