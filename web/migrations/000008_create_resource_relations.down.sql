-- Drops the ownership records. Roll the Go web service back first: it
-- reads and writes this table. Every resource then counts as shared
-- again, and assign-owner has to be run once more after a new up.
DROP TABLE IF EXISTS web_resource_relations;
