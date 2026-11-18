-- Settings per account (docs/design/web-accounts.md section 7): the
-- preferences, pinned groups, saved searches, and a record of each
-- session so an account can list its sessions and end one
-- (decisions/2026-11-18-sessions-list.md).
--
-- Additive: new tables only. Every row belongs to an account and goes
-- with it (accounts are disabled, never deleted, so the cascade only
-- matters for test data).

-- One row per preference an account has set; a preference without a
-- row is the site default. The names are the service's known keys
-- (webapi/internal/settings), the value their JSON encoding, so a new
-- setting needs no migration.
CREATE TABLE web_user_settings (
	web_user_id BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id.',
	name        VARCHAR(64)     NOT NULL COMMENT 'Setting key, e.g. timezone.',
	value       VARCHAR(1024)   NOT NULL COMMENT 'JSON encoding of the value.',
	updated_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP
	                            ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last change.',

	PRIMARY KEY (web_user_id, name),
	CONSTRAINT fk_web_user_settings_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Preferences an account has set.';

-- Groups an account pinned to the top of its menu. The group lives in
-- the logger's schema, so there is no foreign key to it; a pin of a
-- group the account may not see (any more) is kept and not shown.
CREATE TABLE web_pinned_groups (
	web_user_id BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id.',
	group_id    BIGINT          NOT NULL COMMENT 'telegram_groups.id (negative chat id).',
	pinned_at   DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
	                            COMMENT 'When pinned; the pins show in this order.',

	PRIMARY KEY (web_user_id, group_id),
	CONSTRAINT fk_web_pinned_groups_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Groups pinned to an account''s menu.';

-- Searches an account saved: a title and the search page's path and
-- query string on this site.
CREATE TABLE web_saved_searches (
	id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	web_user_id BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id.',
	title       VARCHAR(128)    NOT NULL COMMENT 'Name shown in the list.',
	url         VARCHAR(2048)   NOT NULL COMMENT 'Same-site path and query string.',
	created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (id),
	KEY idx_web_saved_searches_user (web_user_id, id),
	CONSTRAINT fk_web_saved_searches_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Searches saved by an account.';

-- One row per session cookie. The cookie format stays as both web
-- services read it; a session is identified by the SHA-256 of its
-- cookie value, which both can compute. The Go service records a
-- session when it signs one in or first sees its cookie, and keeps
-- last_used_at; both services refuse a cookie whose row is revoked.
-- Rows of an older session_epoch are dead anyway (the cookie's epoch
-- no longer matches) and are removed when the account's sessions are
-- next listed or signed in.
CREATE TABLE web_sessions (
	id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	web_user_id  BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id.',
	token_hash   BINARY(32)      NOT NULL COMMENT 'SHA-256 of the session cookie value.',
	epoch        INT UNSIGNED    NOT NULL COMMENT 'The session_epoch the cookie carries.',
	expires      BIGINT          NOT NULL DEFAULT 0
	                             COMMENT 'The cookie''s expiry, Unix seconds; 0 = never.',
	user_agent   VARCHAR(512)    NOT NULL DEFAULT '' COMMENT 'User-Agent at sign-in.',
	ip           VARCHAR(45)     NOT NULL DEFAULT '' COMMENT 'Address of the last use.',
	signed_in_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'When it started.',
	last_used_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last request seen.',
	revoked_at   DATETIME        NULL DEFAULT NULL
	                             COMMENT 'Ended (signed out); the cookie is refused.',

	PRIMARY KEY (id),
	UNIQUE KEY uq_web_sessions_token (token_hash),
	KEY idx_web_sessions_user (web_user_id, epoch),
	CONSTRAINT fk_web_sessions_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Session cookies of web accounts, for listing and ending them.';
