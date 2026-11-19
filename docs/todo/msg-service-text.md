# msg-service-text

Rebuild the text of the service types that name someone in their
detail but not in their stored line: auto-delete changes, proximity
alerts, shared users, gifts and Premium given, managed bots, checklist
tasks, suggested birthdates and giveaways.

## Why

The daemon's own description of these types names nobody (the viewer
may not see everyone), and webapi/internal/server/service.go rebuilds
only some types with names. So "set messages to auto-delete after
86400 seconds" neither says who nor gives a readable period, a gift
line does not say from whom to whom, and a checklist line does not
quote the tasks.

## Scope

- Go (webapi): rebuild, with the names the viewer may see and their
  refs (and parts, from `msg-service-links`):
  `chat_set_message_auto_delete_time` (who, and the period as "1 day"),
  `proximity_alert_triggered` (traveler, watcher, distance),
  `users_shared`, `gifted_premium`, `gifted_stars`, `gift` (gifter and
  receiver), `managed_bot_created` (the bot), `checklist_tasks_done`
  and `checklist_tasks_added` (the tasks' text), `suggest_birthdate`
  (the date), `giveaway_created` and `giveaway_completed` (prize and
  counts).
- React: nothing beyond what `msg-service-links` draws.

## Design notes

Few of these occur in the archive so far; they are grouped because each
is a small case in the same function. Gift pictures and upgraded gift
details are `msg-gifts`.

## Depends on

Nothing.

## Done when

- Each listed type's line names the people its detail names (when the
  viewer may see them) and links them.
- The auto-delete line gives the period in words.
- A Go test per type.
