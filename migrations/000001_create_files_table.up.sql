-- Files table: a content-addressed store of every file (media) that
-- tgloggerd observes being sent or received through the Telegram client.
--
-- Files are de-duplicated by their SHA-256 digest: when the same content
-- is seen again, `hit_count` is incremented instead of inserting a new
-- row. Other tables reference a file through its surrogate `id`.

CREATE TABLE files (
	-- Surrogate primary key referenced by other tables.
	id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

	-- TDLib persistent remote file identifier (remoteFile.id_).
	tg_file_id  VARCHAR(255)    NOT NULL,

	-- Coarse media category of the file.
	file_type   ENUM('photo', 'video', 'document', 'audio', 'voice', 'sticker', 'animation', 'unknown')
	                            NOT NULL DEFAULT 'unknown',

	-- File size in bytes.
	file_size   BIGINT UNSIGNED NOT NULL DEFAULT 0,

	-- SHA-256 digest of the file content; used to detect duplicates.
	sha256      BINARY(32)      NOT NULL,

	-- Lower-case file extension without the leading dot (e.g. "jpg"); may be unknown.
	file_ext    VARCHAR(10)     NULL,

	-- Number of times a file with this SHA-256 has been observed.
	hit_count   INT UNSIGNED    NOT NULL DEFAULT 1,

	-- Row creation time.
	created_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
	-- Last time the row was updated (e.g. when hit_count was bumped);
	-- NULL until the row is first updated.
	updated_at  TIMESTAMP       NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,

	PRIMARY KEY (id),
	UNIQUE KEY uq_files_tg_file_id (tg_file_id),
	UNIQUE KEY uq_files_sha256 (sha256),
	KEY idx_files_file_type (file_type),
	KEY idx_files_file_size (file_size),
	KEY idx_files_file_ext (file_ext),
	KEY idx_files_hit_count (hit_count),
	KEY idx_files_created_at (created_at),
	KEY idx_files_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Content-addressed store of Telegram files, de-duplicated by SHA-256.';
