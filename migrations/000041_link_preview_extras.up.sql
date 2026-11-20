-- What a link preview card loses today (docs/design/message-types.md
-- section 4.8): the formatting of its description, whether its media is
-- shown above the description, and whether opening the link skips the
-- confirmation. The preview options the sender chose are the message's,
-- not the card's (a message with previews turned off has no card), so
-- they go into the message's meta instead.
--
-- telegram_link_previews has no FULLTEXT index, so these are instant;
-- ALGORITHM=INSTANT makes a server that cannot do that fail the statement
-- instead of copying the table.
ALTER TABLE telegram_link_previews
	ADD COLUMN description_entities JSON NULL DEFAULT NULL
		COMMENT 'Formatting of description; UTF-16 offsets.'
		AFTER description,
	ADD COLUMN show_media_above_description TINYINT(1) NOT NULL DEFAULT 0
		AFTER show_above_text,
	ADD COLUMN skip_confirmation TINYINT(1) NOT NULL DEFAULT 0
		COMMENT 'Opening the link needs no confirmation.'
		AFTER show_media_above_description,
	ALGORITHM=INSTANT;
