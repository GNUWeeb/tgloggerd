# Web backend in Go

Status: decided 2026-11-18, not started. The items are in the "Web
backend in Go" section of [../TODO.md](../TODO.md); the decision is
[decisions/2026-11-18-web-go.md](decisions/2026-11-18-web-go.md).

## 1. Why

The web backend is a Drogon (C++) application. The dashboard is being
redesigned as a React application talking to a JSON API, and the
access-control work of [web-accounts.md](web-accounts.md) (roles,
federations, visibility enforced in every query) is a large amount of
security-critical code. Drogon has cost this project in ways that
matter for both:

- its HTTP client buffers whole bodies and has no Unix sockets, and its
  chunked stream responses send no body to an HTTP/1.0 upstream, the
  way the documented nginx configuration talks to it;
- GCC 13 crashes on a common coroutine pattern used with it;
- sessions, CSRF, tokens, Range handling and filters are hand-rolled;
- the internet-facing part of the system is in a memory-unsafe
  language, and builds are slow.

Speed is not a concern for a low-traffic dashboard. Go gives memory
safety, a standard HTTP server with file serving (Range, If-Range,
conditional requests), plain streaming and Unix sockets, fast builds
and one static binary. The daemons stay C++: they are built around
TDLib.

## 2. Module and dependencies

- The service lives in `webapi/` with the module path
  `tgloggerd/webapi`: no repository host in it, so the code does not
  depend on where any copy of the repository is hosted.
- Third-party modules are kept to a minimum: the standard library's
  HTTP server and router, `database/sql` with a MySQL driver, and
  `golang.org/x/crypto` (Argon2id, BLAKE2b, XChaCha20). They keep their
  upstream import paths and are vendored (`go mod vendor`, `vendor/`
  committed), so a build never fetches anything from the network.
- The Go toolchain version is pinned in the build image.

## 3. Coexistence

The Go service (`tgloggerd-webapi`, its own container on the compose
network) runs beside the Drogon web during the migration:

- nginx sends `/api/` (the new JSON API for the React application) to
  the Go service, and everything else to Drogon, until each part moves;
- both read the same databases with the same users and grants;
- sessions are shared: a session cookie signed by one is valid in the
  other (same `WEB_APP_KEY` derivation, same `session_epoch` check,
  same CSRF scheme), so a user signs in once;
- file tokens (`/files/<token>`) are the same construction, so links
  stay valid when media serving moves;
- the daemon's internal API is called the same way from either.

## 4. Order

1. Service skeleton, build and deployment.
2. Authentication compatible with today's sessions, passwords and
   CSRF, plus the internal API client.
3. OpenAPI description and a generated TypeScript client for the React
   application.
4. The accounts work of web-accounts.md, built in Go from the start.
5. Ports of the existing features, each enforcing the request scope of
   web-accounts.md section 8: media serving, search, chat API, user and
   group pages' data, administration (forwarding, Join chat, contact
   lookup, MCP administration), then the MCP endpoint.
6. Drogon retired once the React application covers every page and
   nothing routes to it.

Rule for every port: same URLs and JSON shapes as the Drogon endpoint
it replaces unless the API is deliberately versioned, and the matching
`web/docs/` page moves or is updated in the same series.
