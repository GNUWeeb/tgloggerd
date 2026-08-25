-- Compare bot tokens as bytes, not as text.
--
-- telegram_bots.token inherited the table's utf8mb4_0900_ai_ci collation, under
-- which comparison is case- and accent-INSENSITIVE -- and the token is both
-- UNIQUE-keyed and the column the web interns bots by. A Bot API token is a
-- case-sensitive credential: two genuinely different tokens that differ only in
-- letter case collide under the old collation, so the second bot's insert takes
-- the duplicate path and silently hands back the FIRST bot's id, wiring a route
-- to the wrong bot with no error anywhere.
--
-- utf8mb4_bin keeps the charset (no data conversion for the existing rows,
-- which are ASCII) and makes equality exact. The UNIQUE key rebuilds under the
-- new collation.
ALTER TABLE telegram_bots
	MODIFY token VARCHAR(255) COLLATE utf8mb4_bin NOT NULL
	COMMENT 'Bot API token used to authenticate a TDLib bot session.';
