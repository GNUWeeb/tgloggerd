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
  Done: migration 000049 (telegram_communities and
  telegram_community_chats; a community has no username in TDLib),
  described in message-types.md section 4.13.
- Go (webapi): resolve the community detail from that table, under the
  same visibility rule the detail has now (a community whose chats the
  viewer may see); OpenAPI and web/docs/message-kinds.md follow.
  Done: ServiceCommunityDetail's community is a ServiceCommunity {id,
  name, photo}, no longer a ChatRef; given once the name is known, to
  a viewer who sees one of its chats or every chat.
- React: draw the new ServiceCommunity: its name as plain text (a
  community has no page, so no /groups link) with a small avatar from
  its photo when there is one; service-detail.test.tsx follows.

## Depends on

Nothing.

## Done when

- A new community service line on real data shows the community's
  name.
- Go tests resolve a stored community and hide one the viewer may not
  see; the daemon stores the community when the line arrives (both
  done).
- The web app draws the community's name and photo.
