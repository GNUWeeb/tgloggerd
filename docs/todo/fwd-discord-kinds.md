# fwd-discord-kinds

Let Discord to Telegram routes tell apart and forward voice messages,
embeds, polls and system messages, each under a flag of its own.

## Why

gwdiscord parses only a message's content, attachments, stickers and
reply reference, so discordd cannot tell a voice message from an audio
file, and a message whose only body is an embed, a poll or a system
event (a member joining, a pin, a boost) is posted as "(no text)"
under the text flag. The fwd-more-types decision
([../design/decisions/2026-11-20-fwd-more-types.md](../design/decisions/2026-11-20-fwd-more-types.md))
left these to this item.

## Scope

- gwdiscord (its own `gwdiscord:` commits, touching only
  src/gwdiscord): parse the message flags (IS_VOICE_MESSAGE), an
  attachment's duration and waveform, `embeds`, `poll` and the message
  `type`.
- discordd (TDLib session): render an embed (title, description, URL),
  a poll (question and options) and a system message as text lines,
  send a voice message as a Telegram voice note, and filter each under
  `voice`, `embed`, `poll` and `system`; stop posting "(no text)".
- Migration: the four flags on `discord_telegram_routes`, `voice`
  copying `fwd_audio`, the other three copying `fwd_text`, so no route
  changes what it forwards.
  Done: gwdiscord, discordd and the migration (000050); what each part
  follows and is sent as is in message-types.md section 4.10.
- Go and React: the new types in the direction's `type_options`, the
  form and the list, as fwd-more-types set them up.

## Depends on

Nothing.

## Done when

- A Discord voice message arrives in Telegram as a voice note, an
  embed or poll as readable text, and each follows its flag; nothing
  is posted as "(no text)".
- gwdiscord's tests parse the new fields; discordd and web tests cover
  the flags.
