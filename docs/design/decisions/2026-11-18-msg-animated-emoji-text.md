# Decision log: 2026-11-18, An animated emoji is stored as a text message

Decided by the TDLib session on 2026-11-18 and approved by the
project owner the same day, for
[../message-types.md](../message-types.md) section 3.1:

- `messageAnimatedEmoji` is stored with `content_type = 'text'` and the
  emoji as its text; `tdlib_type` keeps the class, and the animation
  sticker is not downloaded.
- Reason: on the server it is an ordinary text message of a single
  emoji, which TDLib presents with an animation. Storing it as text
  keeps it in full-text search and in message counts like any other
  text, and an edit that adds more text (which turns it into a
  `messageText`) is then not a change of kind.
