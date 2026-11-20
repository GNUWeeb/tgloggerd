-- Restore the VARCHAR(2048) columns. Values longer than 2048 characters were
-- exactly what the up migration exists to keep, so under strict sql_mode this
-- ALTER fails if any are present; truncate them first, explicitly, rather
-- than letting the ALTER decide.
UPDATE telegram_link_previews SET url = LEFT(url, 2048)
 WHERE CHAR_LENGTH(url) > 2048;
UPDATE telegram_link_previews SET display_url = LEFT(display_url, 2048)
 WHERE CHAR_LENGTH(display_url) > 2048;

ALTER TABLE telegram_link_previews
	MODIFY url         VARCHAR(2048) NOT NULL COMMENT 'The URL the preview describes.',
	MODIFY display_url VARCHAR(2048) NOT NULL DEFAULT '' COMMENT 'URL as shown to the user.';
