# msg-type-labels

Give every content type and every service type a human label, in one
module the chat, the message page, the message lists and the search
filters share.

## Why

The web shows the stored names as they are: the message page's type
pill and Type row say `paid_media` or `video_note`, its "Service
action" row says `chat_add_members`, a service line without text falls
back to the type with underscores turned into spaces, and the message
lists' Type column says `service` for every service row. A reader has
to know the archive's internal names.

## Scope

- React (frontend): one label map for `ContentType` ("Video message",
  "Paid media", ...) and one for the service types the daemon writes
  (message-types.md section 4.2.3, "Members added", "Message pinned",
  ...), with the raw name as the fallback for a service type the map
  does not know yet. Used by: the message page's pill, Type row and
  Service action row (the raw name stays as a monospace hint); the
  chat's fallback service text; the message lists' Type column and
  mobile rows, which show the service type's label for a service row;
  and the content type filter's values.
- Left to other items: the service type and TDLib type filters as
  enums (`msg-list-filters`), the wording of service lines
  (`msg-service-lines`).

## Design notes

`ContentType` labels are a `Record<ContentType, string>`, as the icons
in ui/media.tsx are, so a new content type fails to compile until it
has a label. Service types are not an OpenAPI enum (the daemon adds
them as TDLib adds classes, decision
[2026-11-18-msg-service-type-names](../design/decisions/2026-11-18-msg-service-type-names.md)),
so that map is open-ended.

## Depends on

Nothing.

## Done when

- The message page of a video message shows "Video message" in its
  pill and Type row, and a service message's page shows its action's
  label.
- The group messages list shows each service row's action label in the
  Type column.
- A unit test checks that a known service type gets its label and an
  unknown one its raw name.
