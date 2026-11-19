# Decision log: 2026-11-20, An MCP token is authorised by its owner

Decided by the project owner on 2026-11-20, for the mcp-token-auth
item, amending
[2026-11-20-mcp-tokens-to-superadmin.md](2026-11-20-mcp-tokens-to-superadmin.md):

- The Go service's /mcp accepts a token while its owner is active and,
  for a member owner, still holds the `mcp_tokens` ability; a token
  with no owner record counts as the superadmins'. Disabling the
  account that minted a token no longer disables it; disabling its
  owner does.
- Requests stay attributed to the account that minted the token.
- The Drogon web, which still serves /mcp in production, keeps
  checking the minter only; the rule above takes effect when /mcp
  moves to the Go service.
- Reason: ownership decides who manages a token, so it should decide
  whether the token works; a token handed to the superadmin account,
  or a member's ability withdrawn, must take effect.
