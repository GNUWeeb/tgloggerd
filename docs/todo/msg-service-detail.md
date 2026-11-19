# msg-service-detail

Show what a service message's stored detail says beyond its line: the
new group photo, a video chat's duration and start time, the pinned
message's snippet, a poll option's text, a topic's link and colour, a
community's name.

## Why

The daemon stores each service action's detail in `content_extra`
(message-types.md section 4.2.3), but `ServiceInfo` exposes only the
type and the refs, so these lines stay generic: "changed the group
photo" without the photo, "pinned a message" without what was pinned,
"ended the video chat" without how long it ran, "added an option to a
poll" without the option, "created topic X" without a link to it, and
"a community" without its name.

## Scope

- Go (webapi): `ServiceInfo.detail`, by type (OpenAPI and
  web/docs/message-kinds.md follow):
  - `chat_change_photo`, `suggest_profile_photo`: the photo as
    ChatMedia (the file of role `chat_photo`);
  - `video_chat_ended`: `duration`; `video_chat_scheduled`:
    `start_date`;
  - `pin_message`: a snippet of the pinned message, when it is in the
    archive;
  - `poll_option_added`, `poll_option_deleted`: the option's text;
  - `forum_topic_*`: the topic's id, name and icon colour;
  - `chat_added_to_community`, `chat_removed_from_community`,
    `chat_join_from_community`: the community as a ChatRef, when it is
    in `telegram_groups` and the viewer may see it;
  - `chat_create`, `chat_upgrade_from`: the title (and the basic
    group, linked when known);
  - `chat_set_background`: `only_for_self`.
- React (frontend): the chat's service line and the message page draw
  them: a small thumb of the new photo (opening the viewer), the
  duration and start time, the pinned snippet in quotes, the option
  text, the topic as a link to `?topic=<id>` with its colour dot, the
  community's name linked, "for themselves" for a background only the
  actor sees.
- Left to `msg-chat-backgrounds`: the wallpaper itself and an animated
  photo's video.

## Design notes

Every user and chat named goes through the request scope, as the refs
do. Old rows: pins logged before classification have no stored
message id, so they get no snippet and no link until the repair pass
has re-read them (`msg-repair-census`).

## Depends on

`msg-service-links` (the same ServiceInfo and the same lines).

## Done when

- A `chat_change_photo` line shows the new photo; a `pin_message` line
  quotes the pinned message; a `video_chat_ended` line shows the
  duration; a `forum_topic_created` line links to its topic.
- Go tests cover each type's detail, including a community and a
  pinned message the viewer may not see; React tests cover the drawing.
