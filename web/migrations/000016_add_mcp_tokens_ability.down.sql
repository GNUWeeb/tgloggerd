-- Takes mcp_tokens away from every member first, so the ENUM can lose
-- the value. The members' tokens stay, as they do when an admin takes
-- the ability away.
DELETE FROM web_member_abilities WHERE ability = 'mcp_tokens';
ALTER TABLE web_member_abilities
	MODIFY ability ENUM('forward', 'fetch_files') NOT NULL
	COMMENT 'forward: set up forwarding; fetch_files: fetch unstored files.';
