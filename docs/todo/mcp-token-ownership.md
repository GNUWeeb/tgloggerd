# mcp-token-ownership

Make every MCP token owned by a web user, and let each role manage the
tokens it should: superadmins all tokens, admins their own and their
members' tokens, members only their own.

## Why

Managing MCP tokens takes the admin permission to manage MCP, and the
token page lists and revokes every token, whoever owns it. A member
cannot have a token at all, and an admin can revoke a token that is
not theirs or their members'.

## Scope

The Go part is done: per-role checks on every token route, the
filtered list with owner and minter, minting for another owner,
rename, the superadmins' transfer, the member ability `mcp_tokens`
(web migration 000016), and every existing token given to
`superadmin` (web migration 000015,
[decision](../design/decisions/2026-11-20-mcp-tokens-to-superadmin.md));
webapi/openapi.yaml (McpTokensProps, createMcpToken, renameMcpToken,
transferMcpToken) and web/docs/mcp.md describe it. Left:

- React (frontend): the token page for each role, with the owner and
  minter shown, minting for one of `owners`, rename, and, for
  superadmins (`can_transfer`), transfer; `mcp_tokens` among a
  member's abilities on the accounts page; the MCP menu entry for a
  member holding it opens the index and tokens, not the groups.

## Depends on

Nothing.

## Done when

- A member with `mcp_tokens` mints and revokes its own token on the
  page and sees no other; an admin sees and manages its own and its
  members' tokens; a superadmin sees and manages all and transfers.
- React tests cover the page per role.
