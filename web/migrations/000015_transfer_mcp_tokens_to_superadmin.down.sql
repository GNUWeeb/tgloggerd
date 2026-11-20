-- Gives the MCP tokens back the owners they had before the up
-- migration, read from the audit rows it wrote ("superadmin-transfer"):
-- an owner_transfer row's token goes back to its "from" account, an
-- owner_set row's token loses its owner row (it counts as the
-- superadmins' again). A token whose old owner no longer exists keeps
-- superadmin. Those audit rows are then removed, so a new up writes
-- them afresh. A token's owner changed by hand after the up migration
-- is put back too: roll back only right after deploying.

UPDATE web_resource_relations r
JOIN web_audit a ON a.action = 'owner_transfer' AND a.web_user_id IS NULL
	AND a.detail LIKE 'mcp\_token % from % to % superadmin-transfer'
	AND r.resource_id = CAST(SUBSTRING_INDEX(SUBSTRING_INDEX(a.detail, ' ', 2), ' ', -1) AS SIGNED)
JOIN web_users old ON old.id = CAST(SUBSTRING_INDEX(SUBSTRING_INDEX(a.detail, ' from ', -1), ' ', 1) AS UNSIGNED)
SET r.user_id = old.id, r.created_at = NOW(), r.created_by = NULL
WHERE r.resource_type = 'mcp_token' AND r.relation = 'owner';

DELETE r FROM web_resource_relations r
JOIN web_audit a ON a.action = 'owner_set' AND a.web_user_id IS NULL
	AND a.detail LIKE 'mcp\_token % superadmin-transfer'
	AND r.resource_id = CAST(SUBSTRING_INDEX(SUBSTRING_INDEX(a.detail, ' ', 2), ' ', -1) AS SIGNED)
WHERE r.resource_type = 'mcp_token' AND r.relation = 'owner';

DELETE FROM web_audit WHERE web_user_id IS NULL
	AND action IN ('owner_transfer', 'owner_set')
	AND detail LIKE 'mcp\_token % superadmin-transfer';
