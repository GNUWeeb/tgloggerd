-- Let a link preview's URL be as long as a message allows.
--
-- url and display_url were VARCHAR(2048), but a message can carry 4096
-- characters and a single URL can fill all of them -- presigned, SAML and
-- tracking links routinely blow past 2048. The writer binds these untruncated,
-- and under STRICT_TRANS_TABLES an oversized value does not truncate, it
-- rejects the whole INSERT: the entire preview (title, description, type,
-- the later photo link) was silently lost for that message, and the
-- late-resolution retry failed the same way every time.
--
-- TEXT rather than VARCHAR(4096): neither column is indexed (only site_name
-- is), so nothing constrains the type, and TEXT removes the class of failure
-- instead of moving the cliff. The URL is an identifier, not page metadata --
-- truncating it would destroy the one field that must round-trip exactly.
-- The metadata fields (site_name, title, author) keep their VARCHARs and are
-- clamped by the writer instead, where losing a tail is harmless.
--
-- display_url loses its DEFAULT '' (TEXT cannot carry one); the single writer
-- always binds it explicitly.
ALTER TABLE telegram_link_previews
	MODIFY url         TEXT NOT NULL COMMENT 'The URL the preview describes.',
	MODIFY display_url TEXT NOT NULL COMMENT 'URL as shown to the user.';
