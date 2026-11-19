# msg-service-links

Link the people and chats a service line names: "added Dewi Kusuma and
2 other members" links Dewi Kusuma to her user page, in the chat and on
the message page.

## Why

The API rebuilds a service message's text with the names of the users
and chats it names (those the viewer may see) and sends them as
`ServiceInfo.refs`, and the same text piece by piece as
`ServiceInfo.parts`: `{text, user?, chat?}`, each name pointing at its
index in `refs.users` or `refs.chats` (web/docs/message-kinds.md,
"Service messages"). The web reads neither, so every name is plain
text. It is the most common service line: members added, invited to a
video chat, removed, owner changes, giveaway winners.

## Scope

- React (frontend): the chat's service line and the message page draw
  the parts, each name a link to `/users/<id>` or `/groups/<id>`, and
  fall back to the text when there are no parts. `parts` stays
  optional in the spec: it is absent where the stored text, which
  names nobody, is sent.
- Left to `msg-service-text`: the service types whose text does not
  name anyone yet.

## Design notes

Parts rather than matching names in the text: two people can share a
name, and a name can contain the words around it. A user or chat the
viewer may not see is never named in either (it stays in the count,
"2 other members"), as for the text today. Old rows: service rows
logged before classification have no stored user ids, so they show no
names until the repair pass has re-read them (`msg-repair-census`
counts how many stay without).

## Depends on

Nothing.

## Done when

- A `chat_add_members` line naming a user the viewer may see links to
  that user, in the chat and on the message page; one naming a user
  the viewer may not see shows only the count, with no link and no
  name.
- React tests cover the links.
