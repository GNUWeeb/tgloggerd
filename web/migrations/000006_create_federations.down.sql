-- Drops the federations. Roll the Go web service back first: it reads
-- these tables on every signed-in request. Without them every admin
-- manages every member again.
DROP TABLE web_federation_members;
DROP TABLE web_federation_admins;
DROP TABLE web_federation_groups;
DROP TABLE web_federations;
