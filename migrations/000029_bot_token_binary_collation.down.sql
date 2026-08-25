-- Restore the inherited collation. If two stored tokens differ only by case,
-- this ALTER fails on the UNIQUE key rather than silently merging them --
-- which is the correct failure: those rows were only distinguishable because
-- the up migration made them so.
ALTER TABLE telegram_bots
	MODIFY token VARCHAR(255) COLLATE utf8mb4_0900_ai_ci NOT NULL
	COMMENT 'Bot API token used to authenticate a TDLib bot session.';
