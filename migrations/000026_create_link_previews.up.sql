-- Link previews: the card Telegram renders under a message containing a URL.
--
-- These are resolved by TELEGRAM'S SERVERS, not by the client and not by us.
-- The proof is in the data: a preview's image arrives as a Telegram `file`
-- with a remote id on Telegram's CDN, which only their backend can mint. TDLib
-- never fetches a URL itself. So a preview is a fact the server asserted about
-- a link at a point in time, and it is worth keeping for the same reason the
-- message text is: it is what the reader actually saw, and the linked page can
-- change or vanish afterwards.
--
-- TDLib delivers it as `messageText.link_preview` (td_api::linkPreview). It is
-- discarded entirely today.
--
-- A separate table rather than columns on the message tables. Previews are
-- sparse -- most messages have no link at all -- and the metadata is wide
-- (six text fields plus flags), so inlining it would add mostly-NULL bulk to
-- the hottest table in the schema for the sake of a minority of rows.
--
-- One row per message: td_api::messageText carries exactly one link_preview,
-- for the first previewable URL in the text. The two nullable owner columns are
-- how a row points at either a group or a private message -- the same pair of
-- tables the rest of the schema keeps separate -- and the two unique keys make
-- "one preview per message" a constraint rather than a convention.
--
-- "Exactly one owner is set" is NOT a CHECK constraint, though it reads like
-- one should be. MySQL refuses a CHECK over a column that a foreign key
-- cascades on (error 3823), and the cascade is worth more here: deleting a
-- group already cascades to its messages, so without ON DELETE CASCADE on
-- these the group delete would fail outright. The invariant is upheld by the
-- single writer instead -- DB::upsertLinkPreview sets exactly one -- and a
-- violation would still be caught by the unique keys.
CREATE TABLE telegram_link_previews (
	id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

	-- Exactly one of these is set; see the note above on why not a CHECK.
	group_message_id   BIGINT UNSIGNED NULL
	                                   COMMENT 'FK to telegram_group_messages.id.',
	private_message_id BIGINT UNSIGNED NULL
	                                   COMMENT 'FK to telegram_private_messages.id.',

	-- td_api::linkPreview scalar fields.
	url                VARCHAR(2048)   NOT NULL COMMENT 'The URL the preview describes.',
	display_url        VARCHAR(2048)   NOT NULL DEFAULT '' COMMENT 'URL as shown to the user.',
	site_name          VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'e.g. "GitHub", "YouTube".',
	title              VARCHAR(512)    NOT NULL DEFAULT '',
	description        MEDIUMTEXT      NULL COMMENT 'Preview body; formattedText, stored as plain text.',
	author             VARCHAR(255)    NOT NULL DEFAULT '',

	-- Which td_api::LinkPreviewType arrived, lower-cased without the prefix
	-- ("article", "photo", "chat", "video", ...). Kept as a string rather
	-- than an ENUM: TDLib has forty of these today and adds more with the
	-- API, and an unknown value must be recordable, not a migration.
	type               VARCHAR(48)     NOT NULL DEFAULT 'unsupported',

	-- Non-zero when the page has an Instant View, which is itself a signal
	-- that Telegram parsed the page rather than only scraping og: tags.
	instant_view_version INT           NOT NULL DEFAULT 0,

	-- Display hints the sender or the server chose.
	has_large_media    TINYINT(1)      NOT NULL DEFAULT 0,
	show_large_media   TINYINT(1)      NOT NULL DEFAULT 0,
	show_above_text    TINYINT(1)      NOT NULL DEFAULT 0,

	-- The preview IMAGE, stored like any other file: downloaded, hashed and
	-- content-addressed in telegram_files. NULL when the preview carries no
	-- image, or when it has not been downloaded yet -- the row is written as
	-- soon as the preview is known, and the file linked when it arrives,
	-- exactly as message attachments behave.
	photo_file_id      BIGINT UNSIGNED NULL COMMENT 'FK to telegram_files.id.',

	created_at         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP
	                                   ON UPDATE CURRENT_TIMESTAMP,

	PRIMARY KEY (id),
	UNIQUE KEY uq_link_previews_group (group_message_id),
	UNIQUE KEY uq_link_previews_private (private_message_id),
	KEY idx_link_previews_photo (photo_file_id),
	KEY idx_link_previews_site (site_name),
	CONSTRAINT fk_link_previews_group_msg
		FOREIGN KEY (group_message_id)
		REFERENCES telegram_group_messages (id)
		ON DELETE CASCADE ON UPDATE CASCADE,
	CONSTRAINT fk_link_previews_private_msg
		FOREIGN KEY (private_message_id)
		REFERENCES telegram_private_messages (id)
		ON DELETE CASCADE ON UPDATE CASCADE,
	CONSTRAINT fk_link_previews_photo
		FOREIGN KEY (photo_file_id)
		REFERENCES telegram_files (id)
		ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Server-resolved link preview cards attached to messages.';
