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

- Ownership: every token has an owner (web_mcp_tokens.web_user_id and
  the ownership relation already record one). A token is listed,
  minted, renamed and revoked by its owner, by an admin for a member
  the admin manages (the member grants' rule), and by a superadmin for
  any token. A member mints and manages only their own.
- Existing tokens: all of them are transferred to the `superadmin`
  account, as an audited ownership transfer. This supersedes the
  decision 2026-11-18-mcp-token-owners (existing tokens keep their
  creators); record the new decision.
- Go (webapi): the permission checks above on every token route, a
  token list filtered to what the viewer may manage, an owner column,
  and a transfer action for superadmins; OpenAPI and web/docs/mcp.md
  and accounts.md follow.
- React (frontend): the token page for each role, with the owner
  shown and, for superadmins, transfer.

## Depends on

Nothing.

## Done when

- A member mints and revokes their own token and sees no other; an
  admin sees and manages their own and their members' tokens; a
  superadmin sees and manages all; every existing token is owned by
  `superadmin`, with an audit row for each transfer.
- Go tests cover each role's list and every route's refusals; React
  tests cover the page per role.
