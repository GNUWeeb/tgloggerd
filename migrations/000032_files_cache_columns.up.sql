-- The columns that turn the file store into a cache in front of Telegram
-- (docs/design/storage-cache.md section 3).
--
-- tg_unique_id: TDLib's remoteFile.unique_id, the same for the same file across
-- sessions and accounts (tg_file_id is not). It is the dedup key for files
-- recorded without their bytes, which have no sha256 to dedup on. NULL for
-- rows written before this migration and until a writer fills it in; InnoDB
-- allows any number of NULLs under a UNIQUE key.
--
-- sha256 becomes NULL-able, still UNIQUE: NULL means the bytes were never
-- seen (media recorded from a chat the account is not a member of, or over
-- the per-file cap and never fetched).
--
-- accessed_at: the last time the file was used (ingested again, served by the
-- web). Eviction takes the least recently accessed rows first, through
-- idx_files_evict. Existing rows start from created_at; a one-off seeding
-- script raises that to the latest recorded sign of use at the cutover.
--
-- alias_of: a row whose bytes, once fetched on demand, turn out to equal
-- another row's points at that row instead of taking its sha256 (which is
-- UNIQUE). Readers follow it one hop. This keeps every reference and every
-- minted token to the row valid without repointing the foreign keys of the
-- tables that reference telegram_files.
--
-- on_disk keeps its meaning for 1 but 0 now covers more than "too large":
-- the bytes are not in the store (evicted, never fetched, or over the
-- per-file cap) and can be fetched from Telegram.
--
-- Additive, because the table holds real data. Making sha256 NULL-able
-- rebuilds the table, so everything is done in one ALTER (one rebuild).
ALTER TABLE telegram_files
	ADD COLUMN tg_unique_id VARCHAR(255) NULL DEFAULT NULL
		COMMENT 'TDLib remoteFile.unique_id: stable across sessions and accounts; NULL if unknown.'
		AFTER tg_file_id,
	MODIFY COLUMN sha256 BINARY(32) NULL DEFAULT NULL
		COMMENT 'SHA-256 of the content; NULL = the bytes were never seen.',
	MODIFY COLUMN on_disk BOOLEAN NOT NULL DEFAULT TRUE
		COMMENT 'Whether the file bytes are present in the store; FALSE = not stored (evicted, never fetched, or over the per-file cap), fetchable from Telegram.',
	ADD COLUMN alias_of BIGINT UNSIGNED NULL DEFAULT NULL
		COMMENT 'Row holding the same bytes, followed one hop by readers; NULL if none.'
		AFTER on_disk,
	ADD COLUMN accessed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		COMMENT 'Last use of the file (ingested again, served); drives eviction.'
		AFTER updated_at,
	ADD UNIQUE KEY uq_files_tg_unique_id (tg_unique_id),
	ADD KEY idx_files_evict (on_disk, accessed_at),
	ADD CONSTRAINT fk_files_alias_of
		FOREIGN KEY (alias_of) REFERENCES telegram_files (id)
		ON DELETE SET NULL;

-- Existing rows were last used, as far as is known, when they were created.
-- Keep updated_at: it records the last real change to the row, and this
-- backfill is not one.
UPDATE telegram_files SET accessed_at = created_at, updated_at = updated_at;
