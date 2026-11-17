# go-web-openapi

Describe the Go service's API in OpenAPI and generate the TypeScript
client the React application uses.

## Why

The React application and the Go API must not drift apart; a typed
client generated from one description keeps them in step.

## Scope

- `webapi/openapi.yaml`, kept next to the handlers and checked against
  them in tests (each route described, each described route served).
- A pinned, containerised generator producing the TypeScript client;
  where the client lives is agreed with the frontend work.
- `web/docs/` points at the description as the API contract.

## Design notes

- No network at build time: the generator runs from a pinned image or
  a vendored tool.

## Depends on

- `go-web-auth`

## Done when

- The description covers every Go route, a test fails if a route and
  the description disagree, and the generated client compiles.
