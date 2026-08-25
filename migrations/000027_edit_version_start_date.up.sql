-- Re-key the edit history from "when this version ended" to "when it began".
--
-- A row in telegram_*_message_edits is a superseded version of a message, and
-- it has always carried an edit_date. That date was the edit_date of the
-- version that REPLACED it, because the snapshot was written by the upsert that
-- was in the middle of applying the replacement and that was the date it had in
-- hand. So every version row was stamped with its successor's timestamp, while
-- the live row in telegram_*_messages carried its own. One column, two meanings,
-- depending on which table it was read from.
--
-- The cost of that is not cosmetic. It cannot express when the FIRST version
-- began -- the original text of a message that was later edited has no row
-- describing its own lifetime, only one borrowed from the edit that ended it --
-- and it made the column unusable for ordering a message's versions on a
-- timeline without joining each row to its neighbour.
--
-- It also cannot be written at all by a caller that knows the content but not
-- the new edit_date, which is exactly the position the updateMessageContent
-- handler is in: TDLib delivers the new content inline there, and the edit_date
-- separately, in a different update. Recording a version at the moment its
-- content arrives requires a stamp that does not depend on knowing what will
-- replace it.
--
-- After this, edit_date means the same thing everywhere it appears: the
-- edit_date that this particular version of the message had while it was the
-- live one, with 0 for the original, never-edited version. A message's full
-- history is then its edit rows followed by the live row, each carrying its own
-- start, in id order.
--
-- The rewrite is a shift by one within each message: a version began when the
-- previous version ended, and the earliest one began at 0. LAG() over the rows
-- of one message expresses that directly, and it is exact rather than a
-- reconstruction -- the successor's date is precisely what was stored.

UPDATE telegram_group_message_edits e
JOIN (
	SELECT id,
	       IFNULL(LAG(edit_date) OVER (PARTITION BY group_message_id
					   ORDER BY id), 0) AS started
	FROM telegram_group_message_edits
) x ON x.id = e.id
SET e.edit_date = x.started;

UPDATE telegram_private_message_edits e
JOIN (
	SELECT id,
	       IFNULL(LAG(edit_date) OVER (PARTITION BY private_message_id
					   ORDER BY id), 0) AS started
	FROM telegram_private_message_edits
) x ON x.id = e.id
SET e.edit_date = x.started;
