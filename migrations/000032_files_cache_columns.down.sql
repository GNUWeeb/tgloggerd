-- Refuse to go down while any row has no sha256: NOT NULL cannot be restored
-- over them, and dropping or inventing digests would lose data. The check
-- constraint is evaluated against every existing row, so this statement fails
-- with "Check constraint 'down_000032_needs_every_sha256' is violated" and
-- changes nothing; the statements below never run. migrate then reports the
-- database as dirty at version 31 although the schema is still that of 32:
-- `migrate force 32` records the truth again.
ALTER TABLE telegram_files
	ADD CONSTRAINT down_000032_needs_every_sha256 CHECK (sha256 IS NOT NULL);

ALTER TABLE telegram_files
	DROP CHECK down_000032_needs_every_sha256,
	DROP FOREIGN KEY fk_files_alias_of;

ALTER TABLE telegram_files
	DROP KEY idx_files_evict,
	DROP KEY uq_files_tg_unique_id,
	DROP COLUMN accessed_at,
	DROP COLUMN alias_of,
	DROP COLUMN tg_unique_id,
	MODIFY COLUMN sha256 BINARY(32) NOT NULL,
	MODIFY COLUMN on_disk BOOLEAN NOT NULL DEFAULT TRUE
		COMMENT 'Whether the file bytes are present in the store; FALSE = metadata-only (too large), re-downloadable by tg_file_id.';
