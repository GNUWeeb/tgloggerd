-- Dummy ownership records (web migration 000008), loaded after
-- "accounts" and "admin" (their accounts and MCP tokens are referred
-- to). Invented.
--
-- Rows, beside the fixtures':
--   web_resource_relations  t2d_webhook 1 owned by gw_admin (3), given
--                           by admin (1); d2t_route 1 owned by
--                           intern2025 (7, disabled); mcp_token 1
--                           owned by admin (1), mcp_group
--                           -1001000000001 owned by ops (2). Every
--                           other resource of the logger's "sample" and
--                           "forwarding" fixtures and of "admin" has no
--                           owner.
--   web_users               display name "GW Admin" for gw_admin

UPDATE web_users SET display_name = 'GW Admin' WHERE id = 3;

INSERT INTO web_resource_relations (resource_type, resource_id, relation, user_id,
	created_at, created_by) VALUES
	('t2d_webhook', 1, 'owner', 3, '2026-11-01 10:00:00', 1),
	('d2t_route', 1, 'owner', 7, '2026-11-01 10:00:00', 1),
	('mcp_token', 1, 'owner', 1, '2026-10-01 08:00:00', 1),
	('mcp_group', -1001000000001, 'owner', 2, '2026-11-02 10:00:00', 2);
