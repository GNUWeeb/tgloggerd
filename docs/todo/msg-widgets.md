# msg-widgets

Show a widget message as more than a placeholder: its box at its size,
and its HTML as escaped source on the message page, never rendered.

## Why

A widget is an HTML page of a bot, stored as TDLib's JSON (HTML up to
the cap). The web shows "A widget, which this archive does not show".
No widget appears in the archive so far, so this is low priority.

## Scope

- Daemon (TDLib session): curated `{width, height, widget_id,
  has_caption}` alongside `tdlib`, no migration.
- How the HTML is shown, decided by the owner on 2026-11-20 (decision
  file 2026-11-20-msg-widgets-html): never rendered. The message page
  shows the widget's box at its size with a note, and the HTML as
  escaped source text.
- Go (webapi): a `WidgetDetail` with the size and the HTML as a
  string, sent as text only; nothing serves it as a page.
- React (frontend): the widget's box at its size with a note, and on
  the message page the HTML as escaped source (a code block), never
  through innerHTML or an iframe.

## Design notes

Serving a bot's HTML from the web's own origin would give it the
viewer's session; the sandbox is a security decision, not a storage
one. message-types.md section 3.2.

## Depends on

Nothing.

## Done when

- The decision file exists, and a sample widget shows as it says,
  with no script of the widget running in the web's origin (checked
  in a test of the served headers).
