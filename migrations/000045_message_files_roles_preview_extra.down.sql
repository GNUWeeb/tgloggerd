-- The ENUM cannot shrink while rows hold the new roles: those rows go
-- first (their telegram_files rows stay).
DELETE FROM telegram_message_files
	WHERE role IN ('rich','gift','background','preview_media');

ALTER TABLE telegram_message_files
	MODIFY COLUMN role ENUM('cover','live_video','paid','preview','poll','game','invoice',
		  'chat_photo','thumbnail') NOT NULL
		COMMENT 'What the file is to the message.';

ALTER TABLE telegram_link_previews
	DROP COLUMN extra;
