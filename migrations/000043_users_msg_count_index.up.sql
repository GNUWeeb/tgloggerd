-- The users list is sorted by message count by default
-- (ORDER BY u.msg_count DESC, u.id DESC), and telegram_users had no
-- index on msg_count, so every page of the list sorted all the users
-- first. InnoDB appends the primary key to a secondary index, so
-- (msg_count) serves that order, read backwards, and a page reads only
-- its own rows.
--
-- Additive: one secondary index, built in place.
ALTER TABLE telegram_users
	ADD INDEX idx_users_msg_count (msg_count),
	ALGORITHM=INPLACE;
