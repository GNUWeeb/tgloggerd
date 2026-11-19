# msg-widgets

Show a widget message as more than a placeholder: its size, and its
HTML in a sandbox or behind an "open" action, once a sandbox is chosen
(optional).

## Why

A widget is an HTML page of a bot, stored as TDLib's JSON (HTML up to
the cap). The web shows "A widget, which this archive does not show".
No widget appears in the archive so far, so this is low priority.

## Scope

- Daemon (TDLib session): curated `{width, height, widget_id,
  has_caption}` alongside `tdlib`, no migration.
- Decide how the HTML may be shown: a sandboxed iframe from a separate
  origin with no scripts, a download, or not at all; a decision file.
- Go (webapi): a `WidgetDetail` with the size and, per the decision, a
  URL that serves the HTML with a strict Content-Security-Policy.
- React (frontend): the widget's box at its size, showing the HTML per
  the decision.

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
