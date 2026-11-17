# go-port-chat-api

Port the chat JSON API (`/v1/chats/...`) and the user and group page
data to the Go service.

## Why

The React chat view and the user and group pages need them.

## Scope

- Messages with pagination, jumps, replies, edits, link previews,
  media with `fetchable`, as `web/src/controllers/ApiController.cpp`
  and `web/docs/` describe; the user and group detail and history data
  the current pages render; enforcing the request scope, including
  members seeing users only through their groups.

## Depends on

Nothing.

## Done when

- The chat view's requests give the same answers from both services,
  and scope rules hold for every field.
