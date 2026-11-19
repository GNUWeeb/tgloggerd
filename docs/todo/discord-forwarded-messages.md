# discord-forwarded-messages

Forward the messages that Discord users forward from another channel,
as "Forwarded:" text, so they stop vanishing.

## Why

A Discord forward is a message whose `message_reference` has type
FORWARD and whose content sits in `message_snapshots`: the message
itself has no content, attachments or stickers. gwdiscord does not
parse snapshots, so discordd sees an empty message. It used to post
"(no text)" for it; since fwd-discord-kinds it posts nothing, so a
forward is silently lost in Telegram.

## Scope

- gwdiscord (its own `gwdiscord:` commits, touching only
  src/gwdiscord): parse `message_reference.type` and the first
  snapshot's message (content, attachments, stickers, embeds, and
  its timestamp).
- discordd: render a forward as a "Forwarded:" line followed by the
  snapshot's parts, each under the flag it would follow as an ordinary
  message (text, photo, ..., embed); archive the snapshot's content
  with the message.
- No migration and no new flag: a forward's parts follow the flags of
  their kinds.

## Depends on

`fwd-discord-kinds` (the renderer and the flags it adds).

## Done when

- A forwarded Discord message arrives in Telegram as "Forwarded:" and
  its text and attachment lines, filtered like any other message.
- gwdiscord's test parses a snapshot; discordd's render test covers a
  forward.
