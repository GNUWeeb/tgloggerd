# The dashboard's message series

`GET /api/dashboard/series` counts the messages logged per period over a
range, for the dashboard's chart on `/`. The shape is in
[../../webapi/openapi.yaml](../../webapi/openapi.yaml)
(`getDashboardSeries`); this page explains the behaviour. The Go code is
`webapi/internal/logdb/series.go` and
`webapi/internal/server/dashboardseries.go`.

    GET /api/dashboard/series?period=<p>&start=<unix>&end=<unix|now>

    200 {"period":"hour","start":1794700800,"end":1794877200,"live":true,
         "tz":"Asia/Jakarta","limit":3888000,
         "points":[{"at":1794700800,"n":12},{"at":1794704400,"n":0},...]}
    400 bad_request, with a message: an unknown period, a start or end
        that is not a unix time (or `now`), a start not before the end,
        a time before 1970 or past the year 9999, or a range over the
        period's limit (the message of `dashboard.range_too_long`)

## What is counted

The same messages as the dashboard's per-day bars: the group messages
of the groups the viewer sees (the request scope's group condition) and,
when the viewer sees private chats, the private messages, deleted ones
included. A day series over the bars' 30 days answers the bars' numbers;
a test keeps the two in line.

## Periods

| period | a period starts | longest range |
|---|---|---|
| `minute` | every minute | 24 hours |
| `hour` | every hour | 45 days |
| `4h` | 00:00, 04:00, ... 20:00 | 180 days |
| `day` | midnight | 1096 days |
| `week` | Monday midnight | 5479 days |
| `month` | the 1st, midnight | unlimited |
| `quarter` | 1 January, April, July, October | unlimited |
| `year` | 1 January | unlimited |

All in the viewer's timezone (its timezone preference, else the site's;
`tz` in the answer). Across a daylight saving change a minute and an
hour follow the instant's own UTC offset, so the hour that repeats when
the clocks go back is two points, as the clock showed them; the longer
periods start at their wall-clock time, so the day of a change has 23
or 25 hours, and a 4h period across it 3 or 5.

The limit (`limit`, in seconds, null when unlimited; a day counted as
86400 seconds) applies to the range as asked: end minus start, the end
being the time of the request for `now`. It keeps a chart at about a
thousand points and a query cheap.

## Snapping, zeros and the live end

start is moved back to the start of its period, end forward to the end
of its own (an end exactly on a boundary stays). The points cover every
period in between, oldest first, with zeros for empty periods; `at` is a
period's start. With `end=now` (`live: true`) the series runs to the
end of the period holding the time of the request, so its last point is
the current period, still filling; the client adds the live message
ticks to it and moves the start forward once the range reaches the
limit. Periods that start after the time of the request are zero.

## Cache

A finished period's count is kept in memory, per scope (its group
condition and values and whether it sees private chats), timezone and
period, so asking again counts only the current period. A period is
finished five minutes after its end (a message is logged a moment after
its send time). The cache holds at most 200,000 periods, dropping the
least recently used series first, and forgets a series an hour after it
was first filled, so history logged late shows within the hour.

## Cost

Each request is one query per message table over the range's dates
(`idx_group_messages_date`, or a member's `(chat_id, date, ...)`
ranges), finding each message's period with `INTERVAL()` over the
periods' starts. Only the span from the first period not in the cache
to the current one is counted.
