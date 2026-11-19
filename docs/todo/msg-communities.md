# msg-communities

Store the communities that service lines name, so a "added to the
community" line can give the community's name and photo.

## Why

`chat_added_to_community`, `chat_removed_from_community` and
`chat_join_from_community` carry a TDLib community id. Community ids
are not chat ids, and the daemon stores no communities, so the
`community` detail of `ServiceInfo` (msg-service-detail) only resolves
a community stored under a negative chat id: on real data these lines
get no detail.

## Scope

- Daemon (TDLib session): record each community a message or chat
  names (id, name, photo, username if any) in a new table, filled when
  the line is logged and on demand; a migration for it.
- Go (webapi): resolve the community detail from that table, under the
  same visibility rule the detail has now (a community whose chats the
  viewer may see); OpenAPI and web/docs/message-kinds.md follow.
- React: nothing beyond what msg-service-detail draws, unless the
  community gets a photo (a small avatar beside its name).

## Depends on

`msg-service-detail` (the detail and its drawing).

## Done when

- A new community service line on real data shows the community's
  name.
- Go tests resolve a stored community and hide one the viewer may not
  see; the daemon stores the community when the line arrives.
