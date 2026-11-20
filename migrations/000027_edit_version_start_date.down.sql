-- Restore "edit_date is the date of the version that replaced this one".
--
-- The inverse of the up migration is the same shift in the other direction: a
-- version ended when the next one began, and the newest snapshot ended when the
-- version now live began -- which is the live row's own edit_date, joined in
-- from telegram_*_messages.
--
-- The shift is exact for every row that has a successor snapshot, because that
-- successor's start is precisely the date the row is being given back. It is
-- NOT exact for the newest snapshot of each message, and that is not fixable
-- here: the old encoding recorded the date of the version that replaced it,
-- and if that replacing version was later itself replaced WITHOUT leaving a
-- snapshot -- which is what happens when an edit advances edit_date without
-- changing the content -- then the live row's edit_date has moved past the
-- value being reconstructed, and nothing anywhere still holds it.
--
-- Measured rather than assumed, by running this against the migrated data and
-- diffing it with a copy taken before the up migration: of 1485 rows, 116
-- differed, all 116 of them the newest snapshot of their message, and none of
-- the interior rows. Those 116 come back with the live row's current edit_date
-- instead of the one they had, which is a plausible date in the right order
-- rather than the original value.
--
-- This is a downgrade path, so that is the right trade to take: every version's
-- CONTENT survives intact either way, and only the boundary stamp on the most
-- recent one degrades.

UPDATE telegram_group_message_edits e
JOIN (
	SELECT id, group_message_id,
	       LEAD(edit_date) OVER (PARTITION BY group_message_id
				     ORDER BY id) AS ended
	FROM telegram_group_message_edits
) x ON x.id = e.id
JOIN telegram_group_messages m ON m.id = e.group_message_id
SET e.edit_date = IFNULL(x.ended, m.edit_date);

UPDATE telegram_private_message_edits e
JOIN (
	SELECT id, private_message_id,
	       LEAD(edit_date) OVER (PARTITION BY private_message_id
				     ORDER BY id) AS ended
	FROM telegram_private_message_edits
) x ON x.id = e.id
JOIN telegram_private_messages m ON m.id = e.private_message_id
SET e.edit_date = IFNULL(x.ended, m.edit_date);
