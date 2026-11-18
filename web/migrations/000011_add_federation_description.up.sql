-- A federation's description (docs/design/web-accounts.md section 3):
-- free text its admins and the superadmins keep about it, shown on its
-- details page. At most 2000 characters, newlines and tabs allowed;
-- the web service trims and checks it.
--
-- Additive: one column with a default, so every existing federation
-- has an empty description until someone writes one.
ALTER TABLE web_federations
	ADD COLUMN description VARCHAR(2000) NOT NULL DEFAULT ''
		COMMENT 'Free text about the federation; empty for none.' AFTER name;
