-- Refuse to go down while a lookup journal row is left: it is the only record
-- of a contact that was removed and not yet added back. The check constraint
-- is evaluated against every existing row, so this statement fails with
-- "Check constraint 'down_000033_no_pending_lookup' is violated" when there is
-- one and changes nothing; start the daemon once to re-add the contact, then
-- run `migrate force 33` and go down again.
ALTER TABLE telegram_contact_lookups
	ADD CONSTRAINT down_000033_no_pending_lookup CHECK (FALSE);

DROP TABLE telegram_contact_lookups;

-- first_name/last_name keep the own name. An older daemon overwrites it with
-- the contact name the next time TDLib reports the user, recording the own
-- name in telegram_user_hist_name, as it did before this migration.
ALTER TABLE telegram_users
	DROP COLUMN own_name_checked_at,
	DROP COLUMN own_name_known,
	DROP COLUMN contact_last_name,
	DROP COLUMN contact_first_name,
	DROP COLUMN is_mutual_contact,
	DROP COLUMN is_contact;
