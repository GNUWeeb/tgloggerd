-- MCP bearer tokens for the /mcp tests, on top of "accounts" (load it
-- first). Stored as /mcp stores them: the SHA-256 of the token only.
--
--   1  laptop     tgmcp_1111111111111111111111111111111111111111111111111111111111111111
--                 admin's, live
--   2  old        tgmcp_2222222222222222222222222222222222222222222222222222222222222222
--                 admin's, revoked
--   3  intern     tgmcp_3333333333333333333333333333333333333333333333333333333333333333
--                 intern2025's, whose account is disabled
--
-- Each is owned by its minter (web_resource_relations), as minting
-- records it; /mcp authorises a token by its owner.
--
-- Token ids 1 - 3; a test's own tokens get the next ids.

INSERT INTO web_mcp_tokens (id, web_user_id, name, token_sha256, created_at, last_used_at,
	revoked_at) VALUES
	(1, 1, 'laptop', UNHEX(SHA2('tgmcp_1111111111111111111111111111111111111111111111111111111111111111', 256)), '2026-10-01 00:00:00', NULL, NULL),
	(2, 1, 'old', UNHEX(SHA2('tgmcp_2222222222222222222222222222222222222222222222222222222222222222', 256)), '2026-09-01 00:00:00', NULL,
	 '2026-09-15 00:00:00'),
	(3, 7, 'intern', UNHEX(SHA2('tgmcp_3333333333333333333333333333333333333333333333333333333333333333', 256)), '2026-09-01 00:00:00', NULL, NULL);

INSERT INTO web_resource_relations (resource_type, resource_id, relation, user_id, created_by)
	VALUES ('mcp_token', 1, 'owner', 1, 1), ('mcp_token', 2, 'owner', 1, 1),
	('mcp_token', 3, 'owner', 7, 7);
