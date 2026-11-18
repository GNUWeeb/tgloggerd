-- Indexes for the web pages that read a slice of a big table and then
-- fetched every matching row for one more column:
--
-- - A user's page counts the user's group messages per group, and per
--   day over the last 30 days. With only (sender_user_id) every one of
--   the user's messages was fetched for its chat_id or date (seconds
--   for an active user); (sender_user_id, chat_id) and
--   (sender_user_id, date) answer both from the index alone.
-- - The dashboard counts the users first seen since a time, which had
--   no index on created_at and scanned every user.
-- - The files page and the storage janitor total the stored files' sizes
--   (WHERE on_disk = ?); (on_disk, file_size) answers that from the
--   index alone.
--
-- Additive: secondary indexes only, built in place without copying the
-- tables; reads go on while they build. idx_group_messages_sender_user_id
-- stays (a prefix of the new ones) until nothing relies on it by name.
ALTER TABLE telegram_group_messages
	ADD INDEX idx_group_messages_sender_chat (sender_user_id, chat_id),
	ADD INDEX idx_group_messages_sender_date (sender_user_id, date),
	ALGORITHM=INPLACE;

ALTER TABLE telegram_users
	ADD INDEX idx_users_created_at (created_at),
	ALGORITHM=INPLACE;

ALTER TABLE telegram_files
	ADD INDEX idx_files_on_disk_size (on_disk, file_size),
	ALGORITHM=INPLACE;
