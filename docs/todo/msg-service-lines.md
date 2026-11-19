# msg-service-lines

Make every service line read as a sentence: "You" for the logging
account's own actions, and no actor before a notice that is not
something someone did.

## Why

The chat draws a service message as "<actor> <text>", with no actor
when the message has no sender. For the logging account's own actions
the line then starts with a bare verb ("joined the group", "pinned a
message"). And some texts are not a verb phrase, so an actor in front
reads wrongly: "<actor> the supergroup was created from a group"
(`chat_upgrade_from`), and likewise `chat_upgrade_to`,
`giveaway_completed`, `refunded_upgraded_gift`, the `suggested_post_*`
notices and the expired gift offer.

## Scope

- React (frontend): the chat's service line names the logging account
  "You" when the message has no sender, as the message page already
  does; a list of the service types whose text is a whole sentence
  drops the actor for those and starts the text with a capital.
- The fallback text of a line without stored text uses the label from
  `msg-type-labels`.
- Left to other items: linked names (`msg-service-links`) and detail
  (`msg-service-detail`).

## Design notes

The texts come from the daemon (src/tgloggerd/MessageExtract.cpp,
`describe_service_message`) and, for the types that name people, are
rebuilt for the viewer in webapi/internal/server/service.go. The
sentence types are few and fixed by the daemon's wording, so a list in
the UI is enough; if Go later sends text parts (`msg-service-links`),
the flag can move there.

## Depends on

`msg-type-labels`.

## Done when

- A pin by the logging account shows as "You pinned a message" in the
  chat.
- A `chat_upgrade_from` line shows no actor and starts with a capital.
- Unit tests cover both.
