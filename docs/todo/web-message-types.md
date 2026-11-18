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

Decided with the project owner (2026-11-18), for the chat's message
kinds as the frontend proposed them (ChatMessage.detail by kind,
service refs, interaction, topics):

- Service message text must not carry the names of users or chats the
  viewer may not see: the Go service rebuilds the description for the
  viewer, or removes those names, instead of sending the stored text
  as is; refs leave them out too.
- A live location shows its latest position and the number of
  updates, not a track.
- A contact's phone number follows the rule for users' phones: full
  viewers only.
- Long lists (group call participants, giveaway chats, checklist
  tasks, poll options) are capped at the first 20, with a total.

## Depends on

`msg-topics`

## Done when

- Each new kind renders in the chat view and is searchable.
