# parity-chat

Bring the Go chat data up to what the Drogon chat view shows, before
the new web stack replaces it.

## Why

The parity audit (2026-11-18) found the chat view's media and details
thinner than Drogon's; the stack is deployed only at parity
(decisions/2026-11-18-web-rollout.md and the owner's deploy rule).

## Scope

- Media fields in ChatMessage: video poster or thumbnail, width and
  height, duration, animation, sticker emoji and file extension,
  voice waveform and audio title/performer, from the message's
  content_extra (docs/design/message-types.md section 4.2.1) and the
  files table, falling back to what Drogon reads for older rows.
- Edit history with each version's content type and media.
- Forward origin (id and kind for a link), author signature, service
  type and description as Drogon shows them, link preview URL and
  image.
- Group member counts (not always 0), cross-chat reply targets
  (chat id, so the client can link), a channel or chat posting as a
  sender (not a user link).
- Chat page: `?date=` in the viewer's timezone (date and time
  accepted), old `?after=`, `?after_ts=` and `#msg-N` links answered
  as Drogon did, a way to open the oldest messages, and a
  `/messages/{scope}` path redirecting to the matching list.
- webapi/openapi.yaml exact for every field added.

## Depends on

Nothing.

## Done when

- Every chat feature in the audit's chat rows is served, tested with
  dummy data (testdb), and the contract test passes.
