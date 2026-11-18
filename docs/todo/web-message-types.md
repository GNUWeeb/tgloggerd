# web-message-types

Show the new content kinds, topics, polls and message files in the web
and the search API (the main session's work, on the Go service).

## Why

The daemon stores the detail; readers see it only once the web renders
it.

## Scope

- Chat view rendering per kind from `content_extra`, the message-files
  table, polls and topics; a topic filter for forum groups.
- Search API: the new `content_type` values, `tdlib_type` and
  `service_type` filters; `web/docs/search-api.md`.
- MCP exposure of the new columns decided separately.

## Design notes

message-types.md section 5. Until this lands, the Drogon web shows new
kinds like `unknown`, which it already handles.

## Depends on

`msg-message-files`, `msg-polls`, `msg-topics`

## Done when

- Each new kind renders in the chat view and is searchable.
