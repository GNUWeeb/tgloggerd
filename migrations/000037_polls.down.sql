-- Drops the polls and their options; the messages keep their poll_id in
-- content_extra.
DROP TABLE telegram_poll_options;
DROP TABLE telegram_polls;
