# go-web-pages

Serve pages from the Go service: page routing, one data loader per
route, the render client and the browser-rendered fallback.

## Why

Pages need real URLs whose first response is full HTML, then React
takes over (docs/design/web-go.md section 4).

## Scope

- Page routes in the Go service with the same authentication and
  request scope as the API (404 for hidden things), one loader per
  route returning plain values.
- `GET /api/page/<path>`: the same loader's data as JSON for browser
  navigation, described in the OpenAPI description.
- A client for the render service over a Unix socket (request
  `{route, url, props, viewer settings}`, answer HTML), with a timeout;
  on failure, the application shell with the props embedded.
- HTML assembly: the render service's markup, embedded props safe
  against script injection, static asset links.
- nginx routes each migrated page path to the Go service.

## Design notes

- The render service and its React entry point are the React
  application's (`web/app/`); this item defines and tests the protocol
  with a stand-in renderer.

## Depends on

- `go-web-openapi`

## Done when

- A page answers full HTML with its data embedded through a stand-in
  renderer, the same data comes from `/api/page/<path>`, hidden things
  answer 404, and a stopped renderer gives the shell instead of an
  error.
