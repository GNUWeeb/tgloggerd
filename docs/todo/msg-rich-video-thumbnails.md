# msg-rich-video-thumbnails

Keep the still thumbnail of each video inside a rich message's blocks,
as a main video's is kept.

## Why

A message's own video, animation or video note keeps its still
thumbnail as a `thumbnail` file (message-types.md section 4.3), the
picture the web shows before it plays. The videos and animations
inside a rich message's page blocks are stored under role `rich`
without one, so the web has nothing to show until each video loads.

## Scope

- Daemon (TDLib session): for each video and animation listed under
  role `rich`, its still thumbnail (JPEG, PNG or WebP only, as for a
  main video) as a file of the message, at positions that pair it with
  its video, documented in section 4.3; fetched again like the other
  rich files.
- Go and React: show it as the poster of the rich block's video, in
  the msg-rich-render components.

## Depends on

Nothing.

## Done when

- A new rich message with a video stores the video's still thumbnail,
  and the web shows it before the video plays.
