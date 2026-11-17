# web-live-events

Put the live updates of the Go web service into service: route the
event stream through nginx and check it on the running site.

## Why

Pages should update as things happen, for messages, names, photos and
statistics alike (docs/design/web-live.md). The service side is built:
GET /api/events in webapi/ (webapi/README, "Live updates"), fed by
the daemon's GET /v1/events. What is left needs the deployment.

## Scope

- nginx: the `location = /api/events` block and the
  `large_client_header_buffers` setting of webapi/README ("Routing
  /api/events through nginx"), and HTTP/2 to the browsers if the site
  does not serve it yet.
- Check on the running site that the stream stays open through nginx
  (heartbeats arrive every 15 seconds, events are not held back) and
  that the webapi container reaches the daemon's internal API.

## Depends on

Nothing.

## Done when

- A chat page, a user page and the dashboard update within about a
  second of a new message, a rename and a counter change; a viewer
  never receives events about a group hidden from them.
