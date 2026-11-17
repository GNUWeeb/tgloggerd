# web-live-events

Push live updates to browsers with Server-Sent Events from the Go web
service, filtered by what each viewer may see.

## Why

Pages should update as things happen, for messages, names, photos and
statistics alike (docs/design/web-live.md).

## Scope

- One subscription from the Go service to the daemon's event stream,
  resuming by `seq`, behind an interface a broker could replace later.
- `GET /api/events`: Server-Sent Events for the chats and users the
  page shows, with `Last-Event-ID` resume and a resynchronise event;
  each event checked against the viewer's request scope.
- nginx: buffering off for `/api/events`, HTTP/1.1 upstream.
- The OpenAPI description and the generated client cover it.

## Depends on

- `daemon-event-stream`
- `go-web-auth`

## Done when

- A chat page, a user page and the dashboard update within about a
  second of a new message, a rename and a counter change; a viewer
  never receives events about a group hidden from them.
