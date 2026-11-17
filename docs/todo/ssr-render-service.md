# ssr-render-service

Deploy the Node render service that turns a page's route and data
into HTML for the Go service.

## Why

Server rendering of the React pages needs Node
(docs/design/decisions/2026-11-18-web-ssr.md).

## Scope

- Node, version pinned, in the build image or an image of its own;
  dependencies installed from `web/app/package-lock.json` at build
  time, nothing fetched at run time.
- A `render` compose service as docs/design/web-ssr.md section 8
  describes: the socket directory as its only shared volume,
  `network_mode: none`, read-only file system, non-root, no secrets,
  restarted on failure.
- Health and timing logs; the Go service's fallback covers it being
  down.
- The client bundle (`web/app/dist/client/`) built in the same step
  with the same build id, for the Go service to serve under
  `/app/assets/` (docs/design/web-ssr.md section 6).

## Design notes

- The render entry point itself is built by the React application
  work in `web/app/`.

## Depends on

- `go-web-pages`

## Done when

- The stack renders a page through the service; stopping the service
  makes pages fall back to browser rendering without errors.
