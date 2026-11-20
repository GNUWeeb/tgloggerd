-- Dummy administration rows of the web's schema, loaded after
-- "accounts" (its accounts are referred to). Invented; the token
-- digests are of made-up tokens that open nothing.
--
-- Ids, beside the accounts fixture's:
--   web_mcp_tokens  1 (claude-desktop, admin's, last used), 2 (old
--                   laptop, ops's, revoked); a test's own from 3
--   web_audit       two chat_join attempts by admin, as the Drogon
--                   page wrote them

INSERT INTO web_mcp_tokens (id, web_user_id, name, token_sha256, created_at, last_used_at,
	revoked_at) VALUES
	(1, 1, 'claude-desktop', UNHEX(SHA2('tgmcp_fixture_token_one', 256)),
	 '2026-10-01 08:00:00', '2026-11-17 15:00:00', NULL),
	(2, 2, 'old laptop', UNHEX(SHA2('tgmcp_fixture_token_two', 256)),
	 '2026-03-01 08:00:00', '2026-04-01 08:00:00', '2026-05-01 08:00:00');

INSERT INTO web_audit (web_user_id, action, ip, detail, created_at) VALUES
	(1, 'chat_join', '203.0.113.24', 'joined @gnuweeb "GNU/Weeb"', '2026-11-10 09:00:00'),
	(1, 'chat_join', '203.0.113.24',
	 'flood_wait https://t.me/+AbCdEfGhIjKl "Some Group" retry_after=300',
	 '2026-11-11 09:00:00');
