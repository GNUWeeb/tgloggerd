# mcp-token-auth

Authorise an MCP request by its token's current owner, not by the
account that minted it.

## Why

Since mcp-token-ownership, a token's owner can differ from its minter:
every token moved to the superadmin account, and tokens can be
transferred. The Go /mcp endpoint still checks only that the minter is
active, so a token moved away from a disabled minter keeps working,
and a member who loses the `mcp_tokens` ability keeps its tokens.

## Decision (owner, 2026-11-20)

Fix the Go service's /mcp only. Production /mcp is still served by the
Drogon web, which stays as it is; the fix takes effect when /mcp moves
to the Go service (mcp-switch, after the owner's comparison run).

## Scope

- Go (webapi): token resolution (internal/mcp ResolveToken and its
  callers) checks the owner: the owner account is active, and a
  member owner still holds `mcp_tokens`; a token with no owner row
  counts as the superadmins' (as the token pages already treat it). A
  refused token answers as an unknown one. Requests stay attributed as
  documented in web/docs/mcp.md, which follows.
- No change to the Drogon web.

## Depends on

Nothing.

## Done when

- A token whose owner is disabled, or whose member owner lost
  `mcp_tokens`, is refused by the Go /mcp; one transferred to an active
  owner works although its minter is disabled.
- Go tests cover each case.
