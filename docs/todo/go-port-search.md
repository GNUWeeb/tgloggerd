# go-port-search

Port the advanced search API (`/v1/search/{entity}`) to the Go
service.

## Why

The React application's lists and search pages use it.

## Scope

- The entity-generic field registry, operators, fulltext and
  relevance sort, count estimates on timeout, positional rows, debug
  output, as `web/src/dao/Search.cpp` and `web/docs/search-api.md`
  describe; enforcing the request scope.
- Same JSON shapes; `web/docs/search-api.md` updated.

## Depends on

Nothing.

## Done when

- The same search requests give the same answers from both services,
  and the scope limits every entity's rows and totals.
