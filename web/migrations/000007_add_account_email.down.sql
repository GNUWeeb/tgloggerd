-- Drops the links, the known devices and the verification of the
-- addresses (the addresses stay). Roll the Go web service back first:
-- it reads these on every signed-in request.
DROP TABLE web_known_devices;
DROP TABLE web_email_links;
ALTER TABLE web_users DROP COLUMN email_verified_at;
