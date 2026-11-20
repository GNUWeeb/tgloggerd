# mcp-token-scope

Give each MCP token its own scope: the federations, groups and (for
superadmins only) private chats it may read.

## Why

Every MCP token reads the same thing: every exposed group. A token
handed to one tool or one person cannot be limited to the chats that
tool or person needs.

## Scope

- Design first, as a decision file: what a scope is made of
  (federations, single groups, private chats), how it combines with
  the exposed groups (a scope narrows what a token reads; it never
  reaches a group that is not exposed, except private chats, which
  only a superadmin's token can be given), what a token with no scope
  reads (as today), and how a scope follows changes (a group leaving a
  federation, a group withdrawn from MCP).
- Web migration: the scope rows of a token.
- Go (webapi): the MCP server applies the token's scope to every tool
  and query; the token admin API reads and writes the scope, refusing
  private chats unless the caller is a superadmin; OpenAPI and
  web/docs/mcp.md follow.
- React (frontend): the token form and list show and edit the scope.

## Depends on

Nothing.

## Done when

- A token scoped to one federation reads only that federation's
  exposed groups through every MCP tool; a superadmin's token scoped
  to a private chat reads that chat; an admin cannot give a private
  chat to a token.
- Go tests cover the scope in every MCP tool and the refusals; React
  tests cover the form.
