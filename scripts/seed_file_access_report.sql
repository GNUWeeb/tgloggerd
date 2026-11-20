-- SPDX-License-Identifier: GPL-2.0-or-later
--
-- Report what retention would evict now: the files in the store (on_disk = 1)
-- whose accessed_at is older than the retention period, and their size.
-- Read-only. Use it after scripts/seed_file_access.sql has been applied, or at
-- any later time; that script prints the same figures for its own run (and,
-- as a dry run, for the state seeding would leave).
--
--   docker exec -i tgloggerd-mysql sh -c \
--     'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" tgloggerd' \
--     < scripts/seed_file_access_report.sql
--
-- The period defaults to 90 days (TG_STORAGE_RETENTION_DAYS); another one is
-- given with --init-command="SET @seed_retention_days = <days>". The session
-- runs in UTC, as the daemon's do, so NOW() compares with accessed_at.

SET SESSION time_zone = '+00:00';
SET @seed_retention_days = IF(IFNULL(@seed_retention_days, 0) > 0,
			      @seed_retention_days, 90);

SELECT @seed_retention_days AS retention_days,
       SUM(on_disk = 1) AS on_disk_files,
       ROUND(IFNULL(SUM(IF(on_disk = 1, file_size, 0)), 0) / POW(1024, 3), 2)
	   AS on_disk_gib,
       SUM(on_disk = 1
	   AND accessed_at < NOW() - INTERVAL @seed_retention_days DAY)
	   AS on_disk_older,
       ROUND(IFNULL(SUM(IF(on_disk = 1
			   AND accessed_at < NOW()
				 - INTERVAL @seed_retention_days DAY,
			   file_size, 0)), 0) / POW(1024, 3), 2)
	   AS gib_older
  FROM telegram_files;
