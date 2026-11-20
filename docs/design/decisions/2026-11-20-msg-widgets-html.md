# Decision log: 2026-11-20, A widget's HTML is never rendered

Decided by the project owner on 2026-11-20, for the msg-widgets item
([../message-types.md](../message-types.md) section 3.2):

- A widget message's HTML is never rendered by the web: not inline,
  not in an iframe, not served as a page of its own.
- The chat shows the widget's box at its size with a short note; the
  message page shows the same box and the HTML as escaped source text.
- Reason: a bot's HTML served from the web's origin would run with the
  viewer's session, and a sandbox on a separate origin is more to
  build and keep secure than seeing widgets drawn is worth. Showing
  the source keeps everything the archive stored readable.
