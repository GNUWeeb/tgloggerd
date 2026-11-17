# go-port-mcp

Move the MCP endpoint (`/mcp`) to the Go service.

## Why

It is the last part of the web that only Drogon serves.

## Scope

- The MCP protocol handling (today the in-tree `src/gwmcp` C++
  library), the curated tools of `web/src/mcp/`, bearer tokens, the
  exposure allowlist and the query gate, as `web/docs/mcp.md`
  describes; the same tool names, arguments and answers.

## Design notes

- Keep the curated-exposure rule: nothing readable unless an admin
  exposed the group; one gate used by every query.

## Depends on

- `go-port-chat-api`

## Done when

- An MCP client gets the same tools and answers from the Go service,
  and the exposure gate holds for every tool.
