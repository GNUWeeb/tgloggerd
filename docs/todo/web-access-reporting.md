# web-access-reporting

The web reports which files it serves to the daemon, in batches, and
lets browsers re-check files weekly so that their views count.

## Why

Least-recently-accessed eviction is only as good as its access record.
The web serves the files but may not write `telegram_files`, and its
year-long `immutable` caching hides repeat views entirely.

## Scope

- A small internal-API client in the web (`WEB_TGLOGGERD_API`, bearer
  derived from `WEB_APP_KEY` as the daemon derives it), shared with
  web-fetch-unstored-media.
- A process-wide set of served file ids, flushed every 60 seconds to
  `POST /v1/files/access`, best effort (storage-cache.md section 10).
- `Cache-Control: public, max-age=604800` on `/files/<token>`, without
  `immutable`.
- `.env.example` and `web/README.web`.
- Not in scope: fetching unstored files (web-fetch-unstored-media).

## Design notes

- Drogon's `sendRequestCoro` is fine for these small JSON bodies; run
  the flush from a timer on the main loop and never await it on a
  request path.
- With `WEB_TGLOGGERD_API` empty, nothing is recorded and nothing
  fails.

## Depends on

Nothing.

## Done when

- tgloggerd_web builds.
- At cutover: viewing a file moves its `accessed_at` within about a
  minute, `updated_at` unchanged; stopping the daemon does not break
  file serving.
