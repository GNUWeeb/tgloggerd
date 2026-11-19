-- The member ability mcp_tokens (docs/todo/mcp-token-ownership.md,
-- decisions/2026-11-20-mcp-tokens-to-superadmin.md): a member given it
-- mints and manages its own MCP tokens. Off by default, like the
-- others; superadmins and admins hold it by role.
--
-- Additive: one more value at the end of the ENUM, so the stored rows
-- keep theirs.
ALTER TABLE web_member_abilities
	MODIFY ability ENUM('forward', 'fetch_files', 'mcp_tokens') NOT NULL
	COMMENT 'forward: set up forwarding; fetch_files: fetch unstored files; mcp_tokens: own MCP tokens.';
