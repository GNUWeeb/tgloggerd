-- Every MCP token becomes owned by the first superadmin, the account
-- named `superadmin` (docs/design/decisions/2026-11-20-mcp-tokens-to-superadmin.md,
-- superseding 2026-11-18-mcp-token-owners.md, which left each token
-- with the account that minted it). From here on a token is managed by
-- its owner, by an admin for a member it manages and by every
-- superadmin (web/docs/mcp.md), so a token minted by an account that
-- should no longer manage it starts with the superadmins.
--
-- Data only, no schema change. Ownership is the owner row of
-- web_resource_relations (migration 000008); web_mcp_tokens.web_user_id
-- is not touched: it stays the minter, which the token's requests are
-- attributed to.
--
-- Nothing happens when there is no account named `superadmin` with
-- the superadmin role: every join below is then empty, the tokens keep
-- what they have, and the web service's assign-owner command, which
-- now gives unowned tokens to the account it is told, does the rest.
-- Running this again changes nothing, as every token is then
-- superadmin's already.
--
-- Each change gets the web_audit row the web service's ownership code
-- writes (webdb ownership.go): by no account (as the assign-owner
-- command), with the new owner as the target; owner_transfer with
-- "mcp_token <id> from <old> to <new>" for a token that had another
-- owner, owner_set with "mcp_token <id>" for one that had none, each
-- followed by "superadmin-transfer", which the down migration looks
-- for. The audit rows come first, while the old owners are still
-- there to name.

INSERT INTO web_audit (web_user_id, target_user_id, action, ip, detail)
SELECT NULL, sa.id, 'owner_transfer', NULL,
	CONCAT('mcp_token ', r.resource_id, ' from ', r.user_id, ' to ', sa.id,
		' superadmin-transfer')
FROM web_resource_relations r
JOIN web_users sa ON sa.username = 'superadmin' AND sa.role = 'superadmin'
WHERE r.resource_type = 'mcp_token' AND r.relation = 'owner' AND r.user_id <> sa.id
ORDER BY r.resource_id;

INSERT INTO web_audit (web_user_id, target_user_id, action, ip, detail)
SELECT NULL, sa.id, 'owner_set', NULL, CONCAT('mcp_token ', t.id, ' superadmin-transfer')
FROM web_mcp_tokens t
JOIN web_users sa ON sa.username = 'superadmin' AND sa.role = 'superadmin'
WHERE NOT EXISTS (SELECT 1 FROM web_resource_relations r
	WHERE r.resource_type = 'mcp_token' AND r.resource_id = t.id AND r.relation = 'owner')
ORDER BY t.id;

UPDATE web_resource_relations r
JOIN web_users sa ON sa.username = 'superadmin' AND sa.role = 'superadmin'
SET r.user_id = sa.id, r.created_at = NOW(), r.created_by = NULL
WHERE r.resource_type = 'mcp_token' AND r.relation = 'owner' AND r.user_id <> sa.id;

INSERT INTO web_resource_relations (resource_type, resource_id, relation, user_id, created_by)
SELECT 'mcp_token', t.id, 'owner', sa.id, NULL
FROM web_mcp_tokens t
JOIN web_users sa ON sa.username = 'superadmin' AND sa.role = 'superadmin'
WHERE NOT EXISTS (SELECT 1 FROM web_resource_relations r
	WHERE r.resource_type = 'mcp_token' AND r.resource_id = t.id AND r.relation = 'owner');
