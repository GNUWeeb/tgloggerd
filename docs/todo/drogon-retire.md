# drogon-retire

Remove the Drogon web once nothing routes to it.

## Why

Two web services are a transition, not a destination.

## Scope

- Move what Drogon still does to the Go service: `location = /mcp`
  (after checking the two MCP servers answer alike,
  scripts/mcp_compare.py) and applying the web migrations at its
  container start.
- Confirm the React application covers every page and nginx routes
  nothing to Drogon; then remove `web/` C++ sources, the Drogon and
  MariaDB connector submodules if nothing else uses them, the `web`
  compose service and its build steps; move `web/docs/` and static
  assets where the React application and Go service keep them.

## Depends on

Nothing.

## Done when

- The stack runs without the Drogon web, every documented page and API
  works, and the removed submodules are no longer built.
