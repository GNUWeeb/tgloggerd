# discordd-forward-worker

Take forwarding off the Discord gateway's read thread: each bot's
events go to a bounded worker queue, run in order, with sends to a
Telegram chat paced.

## Why

discordd handles a Discord event inside the gateway callback: it logs
the message, then sends it to every route's Telegram chat and waits
for each send (up to 30 seconds for the send and 30 more for its
confirmation). gwdiscord reads heartbeat acknowledgements on that same
thread, so a callback that blocks past one heartbeat interval (about
41 seconds) makes the gateway close and resume the connection. A burst
in a busy channel, a slow Telegram, or a flood wait already gets
close; the voice notes of fwd-discord-kinds (a download, an upload and
a longer confirmation wait) would make resumes routine. Telegram also
limits a bot to about 20 messages a minute in one group, and today
nothing spaces the sends, so a burst runs into flood waits that end
as send timeouts.

## Scope

- discordd: the gateway callbacks only enqueue a copy of the event.
  One worker per Discord bot runs that bot's events in arrival order,
  so a route's creates, edits and deletes keep their order; bots do
  not wait on each other.
- The archive write moves to the worker with the forward, so a slow
  database cannot stall the gateway either. Each queue is bounded;
  when it is full the event is dropped, neither archived nor
  forwarded, with a warning and a count (the bound is far above any
  normal backlog, so a drop means something downstream is stuck).
- Sends to one Telegram chat are spaced (a minimum gap per chat, longer
  for groups than for private chats), shared by every worker that
  sends to that chat.
- Stopping discordd stops the workers after their current event;
  queued events are dropped with a count in the log (a resumed gateway
  session redelivers what Discord still holds).
- Left to fwd-discord-kinds: once its voice notes run on the worker,
  their confirmation wait no longer stalls the gateway, so its
  fall-back link line after an accepted upload (the double-post risk)
  can go.

## Depends on

Nothing.

## Done when

- No forwarding work runs on the gateway thread; a test with a
  blocking stand-in for Telegram shows the callback returns at once.
- Events of one bot are forwarded in order, bots in parallel, and a
  full queue drops with a warning; unit tests cover the queue and the
  pacer.
