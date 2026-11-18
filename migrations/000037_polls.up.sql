-- Polls and quizzes (docs/design/message-types.md section 4.4). A poll
-- changes after it is sent -- votes arrive, options are added, it closes --
-- and TDLib announces that with updatePoll, keyed by the poll id rather
-- than the message. So a poll is a row of its own, keyed by TDLib's poll id,
-- with its options in a second table; the message points at it from
-- content_extra ("poll_id"). The results are the latest known: they are
-- updated in place, not versioned.
--
-- Only what every reader of the chat sees is kept. What only describes this
-- account -- its own vote, whether it may vote or see the voters, the order
-- its client shows the options in -- is left out.
--
-- New, empty tables: nothing existing is rebuilt.
CREATE TABLE telegram_polls (
	id BIGINT NOT NULL COMMENT 'TDLib poll id.',
	question VARCHAR(1024) NOT NULL DEFAULT '' COMMENT 'Poll question.',
	question_entities JSON NULL DEFAULT NULL
		COMMENT 'Formatting of question; UTF-16 offsets.',
	type ENUM('regular','quiz') NOT NULL DEFAULT 'regular',
	is_anonymous TINYINT(1) NOT NULL DEFAULT 1,
	allows_multiple_answers TINYINT(1) NOT NULL DEFAULT 0,
	allows_revoting TINYINT(1) NOT NULL DEFAULT 0,
	members_only TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Only members of more than a day may vote.',
	country_codes JSON NULL DEFAULT NULL
		COMMENT 'Countries whose users may vote; NULL if any.',
	correct_option_ids JSON NULL DEFAULT NULL
		COMMENT 'Quiz: 0-based positions of the correct options.',
	explanation MEDIUMTEXT NULL COMMENT 'Quiz: the explanation.',
	explanation_entities JSON NULL DEFAULT NULL,
	media JSON NULL DEFAULT NULL
		COMMENT 'The poll media, curated; its file is in telegram_message_files.',
	explanation_media JSON NULL DEFAULT NULL,
	open_period INT NOT NULL DEFAULT 0 COMMENT 'Seconds open after creation; 0 if unlimited.',
	close_date BIGINT NOT NULL DEFAULT 0 COMMENT 'Unix time it closes; 0 if not set.',
	is_closed TINYINT(1) NOT NULL DEFAULT 0,
	total_voter_count INT NOT NULL DEFAULT 0,
	recent_voters JSON NULL DEFAULT NULL
		COMMENT 'Recent voters ({user_id} or {chat_id}), non-anonymous polls only.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Polls and quizzes, with their latest results.';

CREATE TABLE telegram_poll_options (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	poll_id BIGINT NOT NULL COMMENT 'FK to telegram_polls.id.',
	option_id VARCHAR(64) NOT NULL COMMENT 'TDLib option id, unique in the poll.',
	position SMALLINT UNSIGNED NOT NULL DEFAULT 0
		COMMENT '0-based position among the current options.',
	text VARCHAR(1024) NOT NULL DEFAULT '',
	entities JSON NULL DEFAULT NULL,
	media JSON NULL DEFAULT NULL
		COMMENT 'The option media, curated; its file is in telegram_message_files.',
	voter_count INT NOT NULL DEFAULT 0,
	vote_percentage INT NOT NULL DEFAULT 0,
	author_user_id BIGINT NULL DEFAULT NULL COMMENT 'Who added the option, if a user.',
	author_chat_id BIGINT NULL DEFAULT NULL COMMENT 'Who added the option, if a chat.',
	addition_date BIGINT NOT NULL DEFAULT 0 COMMENT 'Unix time it was added; 0 if original.',
	deleted_at DATETIME NULL DEFAULT NULL
		COMMENT 'When it was seen removed from the poll; NULL if present.',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_poll_options_option (poll_id, option_id),
	CONSTRAINT fk_poll_options_poll FOREIGN KEY (poll_id)
		REFERENCES telegram_polls (id)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Options of polls, with their latest vote counts.';
