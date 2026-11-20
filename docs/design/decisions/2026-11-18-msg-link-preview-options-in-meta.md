# Decision log: 2026-11-18, Link preview options belong to the message

Decided by the TDLib session on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.8, which had them
as an `options` column of `telegram_link_previews`:

- The link preview options of a text message (previews turned off,
  another URL, small or large media, shown above the text) are stored
  in the message's `meta` as `link_preview_options`, not on the preview
  card. The card gains only what it describes itself: the formatting of
  its description, whether its media is shown above the description,
  and whether opening the link skips the confirmation.
- Reason: TDLib keeps the options on the message, and a message whose
  sender turned previews off has no card at all, so an `options` column
  on the card could never record the one setting that matters most.
