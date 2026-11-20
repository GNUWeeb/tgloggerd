-- Places for what the lossy message kinds lose today
-- (docs/design/message-types.md sections 4.3 and 4.8):
--
-- telegram_message_files.role gains:
--   rich           the photos, videos and other media inside a rich
--                  message's page blocks (position: order of appearance);
--   gift           a gift's sticker, an upgraded gift's model sticker, the
--                  animation sticker of a gifted premium, stars, grams or
--                  giveaway prize;
--   background     a chat background's document (wallpaper or pattern);
--   preview_media  a link preview's own video, animation or audio, beyond
--                  its image (telegram_link_previews.photo_file_id).
--
-- telegram_link_previews gains extra: the preview type's own fields that
-- have no column (a video's duration, an embedded player's url and size,
-- the target of a message or story preview), as a curated JSON object.
--
-- Nothing writes them yet. Both tables have no FULLTEXT index, and the
-- ENUM append keeps every existing value's number, so both statements are
-- instant; ALGORITHM=INSTANT makes a server that cannot do that fail the
-- statement instead of copying the table.
ALTER TABLE telegram_message_files
	MODIFY COLUMN role ENUM('cover','live_video','paid','preview','poll','game','invoice',
		  'chat_photo','thumbnail','rich','gift','background','preview_media') NOT NULL
		COMMENT 'What the file is to the message.',
	ALGORITHM=INSTANT;

ALTER TABLE telegram_link_previews
	ADD COLUMN extra JSON NULL DEFAULT NULL
		COMMENT 'The preview type''s own fields, curated (message-types.md 4.8); NULL if none.'
		AFTER skip_confirmation,
	ALGORITHM=INSTANT;
