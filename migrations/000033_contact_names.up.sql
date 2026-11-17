-- Keep a user's own name apart from the name the logging account saved them
-- under as a contact.
--
-- TDLib reports a contact's first_name/last_name as the account saved them,
-- not as the user set them. From here on first_name/last_name keep meaning
-- "the user's own name": while the user is a contact they hold the last own
-- name seen before the user became one, and the contact name goes to
-- contact_first_name/contact_last_name. Contact renames are not recorded in
-- telegram_user_hist_name.
--
-- This reverses the rule of the creating migration that the account's
-- relationship to a user is not stored: the contact name is shown as a
-- secondary label in the web, so that a page never passes the account's
-- address book off as the user's name.
--
-- is_contact: td_api::user.is_contact. NULL = not seen since this migration;
-- the daemon fills it in the next time TDLib reports the user. On that first
-- report the daemon also backfills the own name of a user who is a contact
-- from the newest telegram_user_hist_name row whose name differs from the
-- contact name (the name they had before the account saved them).
--
-- own_name_known: 0 when first_name/last_name are only the contact name
-- because no own name was ever seen (the user was a contact before logging
-- began); pages mark the name as such.
--
-- own_name_checked_at: when the admin action "Look up real name" last read
-- the own name from Telegram (it removes the contact, reads the name, adds
-- the contact back).
ALTER TABLE telegram_users
	ADD COLUMN is_contact TINYINT(1) NULL DEFAULT NULL
		COMMENT 'The logging account has the user as a contact; NULL = not seen since migration 000033.'
		AFTER last_name,
	ADD COLUMN is_mutual_contact TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'The user also has the logging account as a contact.'
		AFTER is_contact,
	ADD COLUMN contact_first_name VARCHAR(255) NULL DEFAULT NULL
		COMMENT 'First name the account saved the contact under; NULL if not a contact.'
		AFTER is_mutual_contact,
	ADD COLUMN contact_last_name VARCHAR(255) NULL DEFAULT NULL
		COMMENT 'Last name the account saved the contact under; NULL if not a contact.'
		AFTER contact_first_name,
	ADD COLUMN own_name_known TINYINT(1) NOT NULL DEFAULT 1
		COMMENT '0 = first_name/last_name are the contact name, no own name was ever seen.'
		AFTER contact_last_name,
	ADD COLUMN own_name_checked_at DATETIME NULL DEFAULT NULL
		COMMENT 'Last time the own name was looked up by removing and re-adding the contact.'
		AFTER own_name_known;

-- The journal of the "Look up real name" cycle. A row holds the contact
-- exactly as the account has it and is written before the contact is removed;
-- it is deleted only after the contact was added back. A row left over (the
-- daemon died between removal and re-add) is re-added when the daemon starts,
-- so a contact is never lost.
CREATE TABLE telegram_contact_lookups (
	user_id            BIGINT       NOT NULL COMMENT 'FK to telegram_users.id: the contact being looked up.',
	phone_number       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'Phone number of the contact, if the account knows it.',
	first_name         VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Contact first name, as saved by the account.',
	last_name          VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Contact last name, as saved by the account.',
	note               JSON         NULL COMMENT 'userFullInfo.note as {"text": ..., "entities": [...]}; NULL if none.',
	share_phone_number TINYINT(1)   NOT NULL DEFAULT 0 COMMENT 'The account shares its phone number with the contact.',
	removed_at         DATETIME     NULL DEFAULT NULL COMMENT 'When the contact was removed; NULL = not removed yet.',
	attempts           INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Failed re-add attempts.',
	last_error         VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Last re-add error, for the log and the web.',
	created_at         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'When the cycle started.',

	PRIMARY KEY (user_id),
	CONSTRAINT fk_contact_lookups_user
		FOREIGN KEY (user_id) REFERENCES telegram_users (id)
		ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Contacts removed for a real-name lookup and not yet added back.';
