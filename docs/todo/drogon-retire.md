# drogon-retire

Remove the Drogon web once nothing routes to it.

## Why

Two web services are a transition, not a destination.

## Scope

- Confirm the React application covers every page and nginx routes
  nothing to Drogon; then remove `web/` C++ sources, the Drogon and
  MariaDB connector submodules if nothing else uses them, the `web`
  compose service and its build steps; move `web/docs/` and static
  assets where the React application and Go service keep them.

## Depends on

- `go-port-media`
- `go-port-mcp`

## Done when

- The stack runs without the Drogon web, every documented page and API
  works, and the removed submodules are no longer built.
