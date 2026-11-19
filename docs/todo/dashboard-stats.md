# dashboard-stats

Let the dashboard's message chart on `/` count per minute, hour, four
hours, day, week, month, quarter or year, over a chosen start and end
(the end may be "now", which keeps the chart live), drawn as bars, a
line, an area or a running total.

## Why

The chart shows messages logged per day for the last 30 days only. A
busy hour, a quiet night or a year's growth cannot be seen.

## Decisions (main session, at the owner's request to decide)

- Periods: minute, hour (the default), 4 hours, day, week (starting
  Monday), month, 3 months (calendar quarters), year; all in the
  viewer's timezone, as the per-day bars are today.
- Range: a start and an end, picked down to the second; the picker
  shows the precision the period needs (a per-minute chart picks
  minutes, a per-month chart months).
- The longest range per period, so a chart has at most about a
  thousand points and a query stays cheap:

  | Period | Longest range |
  |---|---|
  | minute | 24 hours |
  | hour | 45 days |
  | 4 hours | 180 days |
  | day | 3 years |
  | week | 15 years |
  | month | all time |
  | 3 months | all time |
  | year | all time |

  A range over the limit is refused by the API and clamped by the
  picker, which moves the start.
- End "now": the end can be set to **Now** (the word on the control).
  The chart then follows the clock: its last period fills as messages
  arrive (through the dashboard's live message ticks), and when the
  range reaches the period's limit the start moves forward with the
  end. A fixed end is not live.
- Default: per hour, the last 48 hours, ending now.
- Variants: **Bars** (the default, as today), **Line**, **Area** (the
  line filled), and **Running total** (each point the sum so far, as a
  line). Each draws the same series; the choice is remembered per
  viewer with the period and range.

## Scope

- Go (webapi): `GET /api/dashboard/series` with `period`, `start`,
  `end` (a unix time, or `now`) and the viewer's timezone: the counts
  per period over the range, visible messages only as the per-day bars
  count them today, with the bucket starts. The longest range per
  period is enforced. Finished periods are cached (they never change);
  the current one is counted fresh. Check the query plan on the
  largest range of each period; if a range is too slow, lower its
  limit here rather than add a table. OpenAPI and web/docs follow.
- React (frontend): the period select, the start/end pickers with
  **Now**, the variant select, the clamped range, live updates while
  the end is now, and the four drawings, accessible (a table view of
  the numbers for screen readers).
- The current per-day KPIs and tiles stay as they are.

## Depends on

Nothing.

## Done when

- Each period draws over its full allowed range within the dashboard's
  usual answer time, and a range over the limit is refused.
- With the end at Now, the last bar grows as messages arrive and the
  start moves once the limit is reached.
- Go tests cover every period's bucketing across a daylight saving
  change, the limits and the cache; React tests cover the controls,
  the variants and the live end.
