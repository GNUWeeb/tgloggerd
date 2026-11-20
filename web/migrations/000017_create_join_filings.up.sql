-- Held filings of joined groups (join-chat-federations): whoever joins
-- a chat from the Join chat page picks the federations the group goes
-- into. When the chat must first approve the join (a request sent, a
-- guard bot to satisfy), the choice waits here until the account is a
-- member, and is then filed into web_federation_groups and dropped.
--
-- Additive: a new table only. chat_id is the logger's (negative) chat
-- id, in another schema, so there is no foreign key to it; it is 0
-- when the account could not see the chat before joining (a private
-- group behind an invite link), and the row is then matched by ref.
-- A filing goes with its federation and with the account that asked.
CREATE TABLE web_join_filings (
	id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	web_user_id   BIGINT UNSIGNED NOT NULL COMMENT 'web_users.id of the account that asked.',
	federation_id BIGINT UNSIGNED NOT NULL COMMENT 'web_federations.id, never 0.',
	ref           VARCHAR(256)    NOT NULL COMMENT 'The reference as the daemon normalised it.',
	chat_id       BIGINT          NOT NULL DEFAULT 0
	                              COMMENT 'telegram_groups.id (negative chat id); 0 if unknown.',
	title         VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'The chat title at the request.',
	msg_count     BIGINT UNSIGNED NOT NULL DEFAULT 0
	                              COMMENT 'telegram_groups.msg_count at the request; more means a member.',
	created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (id),
	UNIQUE KEY uq_web_join_filings (ref, federation_id),
	KEY idx_web_join_filings_chat (chat_id),
	KEY idx_web_join_filings_user (web_user_id),
	CONSTRAINT fk_web_join_filings_user FOREIGN KEY (web_user_id)
		REFERENCES web_users (id) ON DELETE CASCADE,
	CONSTRAINT fk_web_join_filings_fed FOREIGN KEY (federation_id)
		REFERENCES web_federations (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Federations a group awaiting join approval goes into (Join chat).';
