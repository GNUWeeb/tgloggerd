-- Verified email addresses, one-time links and known devices
-- (docs/design/web-accounts.md section 8).
--
-- Additive: one column and two new tables. The addresses recorded
-- since migration 000004 stay as they are, unverified: an account
-- verifies its address with a link (or by setting its password from
-- an invitation sent to it) before it is used for resets and notices.

ALTER TABLE web_users
	ADD COLUMN email_verified_at DATETIME NULL DEFAULT NULL
		COMMENT 'When email was verified; NULL while it is not (or there is none).'
		AFTER email;

-- One row per link sent: a set-password link (an invitation or a
-- reset) or an address verification link. Only the SHA-256 of the
-- token is kept; the token itself is in the email alone. A link works
-- once (used_at) and until it expires; sending a new link of the same
-- purpose to an account ends its earlier ones (used_at as well).
CREATE TABLE web_email_links (
	id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	web_user_id BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id.',
	purpose     ENUM('password', 'verify') NOT NULL
	                            COMMENT 'password: set the password; verify: verify email.',
	token_hash  BINARY(32)      NOT NULL COMMENT 'SHA-256 of the token in the link.',
	email       VARCHAR(254)    NOT NULL COMMENT 'The address the link was sent to.',
	created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at  DATETIME        NOT NULL COMMENT 'The link does not work after this.',
	used_at     DATETIME        NULL DEFAULT NULL
	                            COMMENT 'Used, or replaced by a newer link; works no more.',

	PRIMARY KEY (id),
	UNIQUE KEY uq_web_email_links_token (token_hash),
	KEY idx_web_email_links_user (web_user_id, purpose, used_at),
	CONSTRAINT fk_web_email_links_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='One-time links sent by email, by the hash of their token.';

-- The devices an account has signed in from, by the SHA-256 of their
-- description ("Firefox on Linux"), so a sign-in from another one
-- sends a notice. Session records go when the epoch moves on; these
-- stay.
CREATE TABLE web_known_devices (
	web_user_id   BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id.',
	device_hash   BINARY(32)      NOT NULL COMMENT 'SHA-256 of the device description.',
	first_seen_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (web_user_id, device_hash),
	CONSTRAINT fk_web_known_devices_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Devices an account signed in from, for the new device notice.';
