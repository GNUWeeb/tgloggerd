# go-port-media

Switch `/files/<token>` over to the Go service, which serves it
already.

## Why

Media serving is the busiest endpoint, and Go's standard file serving
handles Range and conditional requests natively (docs/design/web-go.md).
The Go handler (`webapi/internal/server/media.go`) is written and
tested against a fake daemon; what is left needs the live daemon, the
production store and nginx.

## Scope

- Send the same requests to both services, the Drogon web and the Go
  service on its own address, before switching: every case in
  web/docs/media.md (stored file with and without Range, a document's
  download name, an alias row, an unknown or forged token, an unstored
  small file anonymously and signed in, a big file's partial answer
  with seeking, a big file anonymously, the daemon stopped), comparing
  status codes and headers. The expected differences are listed in
  web/docs/media.md, "Differences between the two".
- Check that a served file's id reaches the daemon's eviction record
  (`accessed_at`) within a couple of minutes, and that a signed-in
  request writes its `media` audit row.
- nginx routes `/files/` to the Go service (webapi/README, "Routing
  /files/ through nginx"); web/README.web's nginx example and
  web/docs/media.md say that it does.

## Depends on

Nothing.

## Done when

- Every case in web/docs/media.md behaves the same from the Go
  service, checked with the same requests against both before
  switching nginx over, and nginx sends `/files/` to it.
