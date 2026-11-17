-- SPDX-License-Identifier: GPL-2.0-or-later
--
-- Seed telegram_files.accessed_at from the recorded signs of use
-- (docs/design/storage-cache.md section 9, "Seeding").
--
-- Migration 000032 started every file at accessed_at = created_at, because
-- nothing recorded access before it. Retention evicts by accessed_at, so
-- without this the first sweep would evict files people did look at. This
-- raises accessed_at to the latest of:
--
--   1. the web audit: tgloggerd_web.web_audit rows with action = 'media' and
--      detail = 'file <id>' (an authenticated view of /files/<token>, written
--      by web/src/controllers/MediaController.cpp). detail is free text, so a
--      row counts only if it matches exactly; anything else is ignored;
--   2. the newest message that references the file through file_id, in
--      telegram_group_messages, telegram_private_messages and their *_edits
--      tables: the later of the message's date and its edit_date (an edit
--      row's edit_date is when that version began, migration 000027), so a
--      file still being posted or edited around counts as alive;
--   3. Discord media forwards: telegram_discord_sent_messages rows of
--      kind = 'media' (the forwarder's record of what it posted), at their
--      created_at, for the message that references the file;
--   4. link preview images: telegram_link_previews.photo_file_id, at the
--      date of the message the preview belongs to (group_message_id or
--      private_message_id), by the same rule as 2;
--   5. current avatars: a file that is the current
--      telegram_users.profile_photo_file_id or telegram_groups.photo_file_id
--      is shown on every page about that user or group, so it counts as
--      used now: NOW(), the time of the run. Without this, retention would
--      evict every current avatar at once. The history tables
--      (telegram_user_hist_profile_photo, telegram_group_hist_photo) are not
--      a signal: an old avatar keeps only the signals above.
--
-- Guarantees:
--
--   * accessed_at is only ever raised, never lowered, and never set past
--     NOW() (a signal dated in the future is ignored). updated_at is kept
--     (updated_at = updated_at): this is not a change to the row.
--   * Idempotent for signals 1-4: a second run finds nothing newer and
--     changes nothing. Signal 5 is NOW() and so, by design, raises the
--     current avatars again on every run, but only those whose accessed_at
--     is more than a minute old: a rerun within a minute changes nothing at
--     all, and a later one touches only current avatars.
--   * Safe while the daemon and the web run: each UPDATE re-checks the row's
--     current accessed_at, so an access recorded meanwhile is kept.
--   * Short transactions: files are processed in id ranges of @seed_batch
--     (default 10000) ids, each range committed on its own. The message
--     tables are reached through their file_id indexes; the audit is read
--     once into a temporary table.
--
-- It creates the procedure tgloggerd.seed_file_access_run for the batch loop
-- and drops it again at the end; it touches no other table than
-- telegram_files, and that one only when applying.
--
-- How to run
-- ----------
--
-- As MySQL root (it reads both the tgloggerd and the tgloggerd_web schema),
-- with tgloggerd as the default database. If the schemas are named otherwise
-- (MYSQL_DATABASE / WEB_DB_APP_NAME in .env), edit the names below.
--
-- Dry run (the default): changes nothing, prints how many files would be
-- raised and how many on-disk files and GiB would still be older than the
-- retention period afterwards:
--
--   docker exec -i tgloggerd-mysql sh -c \
--     'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" tgloggerd' \
--     < scripts/seed_file_access.sql
--
-- Apply: the same with @seed_apply = 1:
--
--   docker exec -i tgloggerd-mysql sh -c \
--     'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" \
--        --init-command="SET @seed_apply = 1" tgloggerd' \
--     < scripts/seed_file_access.sql
--
-- Optional settings, through --init-command the same way:
--   @seed_batch           ids per batch (default 10000)
--   @seed_retention_days  retention period of the report (default 90, as
--                         TG_STORAGE_RETENTION_DAYS)
--
-- scripts/seed_file_access_report.sql prints the same report for the current
-- state at any later time.
--
-- Time zone: the daemon's sessions run in UTC (src/mysql/ConnectionPool.cpp),
-- so accessed_at, created_at and the Discord rows are UTC; this session is set
-- to UTC so FROM_UNIXTIME() and NOW() agree with them. web_audit.created_at is
-- written in the web's session time zone, which is the server default; check
-- that `SELECT @@global.time_zone, @@system_time_zone` is UTC before applying.

SET SESSION time_zone = '+00:00';
-- Plain reads of the message tables, no shared locks on them.
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
SET SESSION autocommit = 1;

SET @seed_apply = IF(IFNULL(@seed_apply, 0) <> 0, 1, 0);
SET @seed_batch = IF(IFNULL(@seed_batch, 0) > 0, @seed_batch, 10000);
SET @seed_retention_days = IF(IFNULL(@seed_retention_days, 0) > 0,
			      @seed_retention_days, 90);
SET @seed_raised = 0;

-- Signal 1, once: the newest authenticated view per file id. Only details of
-- exactly "file <decimal id>" count; the case-sensitive match rejects
-- "File 1", "file 12abc", "file -3", "file " and the like, which a bare
-- CAST would turn into ids.
DROP TEMPORARY TABLE IF EXISTS seed_audit;
CREATE TEMPORARY TABLE seed_audit (
	file_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
	last_at DATETIME        NOT NULL
) ENGINE=InnoDB;
INSERT INTO seed_audit (file_id, last_at)
SELECT CAST(SUBSTRING(detail, 6) AS UNSIGNED), MAX(created_at)
  FROM tgloggerd_web.web_audit
 WHERE action = 'media'
   AND REGEXP_LIKE(detail, '^file [1-9][0-9]{0,18}$', 'c')
 GROUP BY 1;

-- Per batch: the latest signal for each file in the id range.
DROP TEMPORARY TABLE IF EXISTS seed_batch;
CREATE TEMPORARY TABLE seed_batch (
	file_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
	last_at DATETIME        NOT NULL
) ENGINE=InnoDB;

-- Dry run: what would be raised, for the report.
DROP TEMPORARY TABLE IF EXISTS seed_preview;
CREATE TEMPORARY TABLE seed_preview (
	file_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
	last_at DATETIME        NOT NULL
) ENGINE=InnoDB;

DROP PROCEDURE IF EXISTS seed_file_access_run;

DELIMITER //
CREATE PROCEDURE seed_file_access_run(IN p_apply TINYINT, IN p_batch BIGINT)
BEGIN
	DECLARE lo BIGINT UNSIGNED;
	DECLARE hi BIGINT UNSIGNED;
	DECLARE max_id BIGINT UNSIGNED;

	SELECT IFNULL(MIN(id), 1), IFNULL(MAX(id), 0)
	  INTO lo, max_id
	  FROM telegram_files;

	WHILE lo <= max_id DO
		SET hi = lo + p_batch - 1;

		DELETE FROM seed_batch;
		INSERT INTO seed_batch (file_id, last_at)
		SELECT s.file_id, MAX(s.t)
		  FROM (
			SELECT m.file_id,
			       FROM_UNIXTIME(GREATEST(m.date, m.edit_date)) AS t
			  FROM telegram_group_messages m
			 WHERE m.file_id BETWEEN lo AND hi
			UNION ALL
			SELECT e.file_id,
			       FROM_UNIXTIME(GREATEST(m.date, e.edit_date))
			  FROM telegram_group_message_edits e
			  JOIN telegram_group_messages m
			    ON m.id = e.group_message_id
			 WHERE e.file_id BETWEEN lo AND hi
			UNION ALL
			SELECT m.file_id,
			       FROM_UNIXTIME(GREATEST(m.date, m.edit_date))
			  FROM telegram_private_messages m
			 WHERE m.file_id BETWEEN lo AND hi
			UNION ALL
			SELECT e.file_id,
			       FROM_UNIXTIME(GREATEST(m.date, e.edit_date))
			  FROM telegram_private_message_edits e
			  JOIN telegram_private_messages m
			    ON m.id = e.private_message_id
			 WHERE e.file_id BETWEEN lo AND hi
			UNION ALL
			SELECT m.file_id, d.created_at
			  FROM telegram_group_messages m
			  JOIN telegram_discord_sent_messages d
			    ON d.telegram_chat_id = m.chat_id
			   AND d.telegram_message_id = m.message_id
			   AND d.kind = 'media'
			 WHERE m.file_id BETWEEN lo AND hi
			UNION ALL
			SELECT m.file_id, d.created_at
			  FROM telegram_private_messages m
			  JOIN telegram_discord_sent_messages d
			    ON d.telegram_chat_id = m.chat_id
			   AND d.telegram_message_id = m.message_id
			   AND d.kind = 'media'
			 WHERE m.file_id BETWEEN lo AND hi
			UNION ALL
			SELECT p.photo_file_id,
			       FROM_UNIXTIME(GREATEST(m.date, m.edit_date))
			  FROM telegram_link_previews p
			  JOIN telegram_group_messages m
			    ON m.id = p.group_message_id
			 WHERE p.photo_file_id BETWEEN lo AND hi
			UNION ALL
			SELECT p.photo_file_id,
			       FROM_UNIXTIME(GREATEST(m.date, m.edit_date))
			  FROM telegram_link_previews p
			  JOIN telegram_private_messages m
			    ON m.id = p.private_message_id
			 WHERE p.photo_file_id BETWEEN lo AND hi
			UNION ALL
			-- Current avatars: now, unless accessed within the last
			-- minute, so an immediate rerun changes nothing.
			SELECT u.profile_photo_file_id, NOW()
			  FROM telegram_users u
			  JOIN telegram_files f ON f.id = u.profile_photo_file_id
			 WHERE u.profile_photo_file_id BETWEEN lo AND hi
			   AND f.accessed_at < NOW() - INTERVAL 1 MINUTE
			UNION ALL
			SELECT g.photo_file_id, NOW()
			  FROM telegram_groups g
			  JOIN telegram_files f ON f.id = g.photo_file_id
			 WHERE g.photo_file_id BETWEEN lo AND hi
			   AND f.accessed_at < NOW() - INTERVAL 1 MINUTE
			UNION ALL
			SELECT a.file_id, a.last_at
			  FROM seed_audit a
			 WHERE a.file_id BETWEEN lo AND hi
		  ) s
		 -- A signal from the future (a message date off by clock skew)
		 -- is ignored rather than clamped to NOW(): clamping would raise
		 -- the file again on every run.
		 WHERE s.t <= NOW()
		 GROUP BY s.file_id;

		IF p_apply THEN
			UPDATE telegram_files f
			  JOIN seed_batch s ON s.file_id = f.id
			   SET f.accessed_at = s.last_at,
			       f.updated_at = f.updated_at
			 WHERE s.last_at > f.accessed_at;
			SET @seed_raised = @seed_raised + ROW_COUNT();
		ELSE
			INSERT INTO seed_preview (file_id, last_at)
			SELECT s.file_id, s.last_at
			  FROM seed_batch s
			  JOIN telegram_files f ON f.id = s.file_id
			 WHERE s.last_at > f.accessed_at;
			SET @seed_raised = @seed_raised + ROW_COUNT();
		END IF;

		SET lo = hi + 1;
	END WHILE;
END//
DELIMITER ;

-- Retention as it stands before this run, for comparison.
SELECT COUNT(*), ROUND(IFNULL(SUM(file_size), 0) / POW(1024, 3), 2)
  FROM telegram_files
 WHERE on_disk = 1
   AND accessed_at < NOW() - INTERVAL @seed_retention_days DAY
  INTO @seed_old_files_before, @seed_old_gib_before;

CALL seed_file_access_run(@seed_apply, @seed_batch);
DROP PROCEDURE seed_file_access_run;

-- After applying, seed_preview is empty and this is the state of the table;
-- after a dry run it is the state the table would have.
SELECT IF(@seed_apply, 'applied', 'dry run (set @seed_apply = 1 to apply)')
	   AS mode,
       @seed_raised AS files_raised,
       @seed_retention_days AS retention_days,
       @seed_old_files_before AS on_disk_older_before,
       @seed_old_gib_before AS gib_older_before,
       COUNT(*) AS on_disk_older_after,
       ROUND(IFNULL(SUM(f.file_size), 0) / POW(1024, 3), 2) AS gib_older_after
  FROM telegram_files f
  LEFT JOIN seed_preview p ON p.file_id = f.id
 WHERE f.on_disk = 1
   AND GREATEST(f.accessed_at, IFNULL(p.last_at, f.accessed_at))
       < NOW() - INTERVAL @seed_retention_days DAY;

DROP TEMPORARY TABLE seed_preview;
DROP TEMPORARY TABLE seed_batch;
DROP TEMPORARY TABLE seed_audit;
