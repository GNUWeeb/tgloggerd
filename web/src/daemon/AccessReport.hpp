// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD_WEB_DAEMON_ACCESSREPORT_HPP
#define TGLOGGERD_WEB_DAEMON_ACCESSREPORT_HPP

#include <cstdint>

namespace tgweb::daemon::access {

/*
 * Report the files the web serves to the daemon, which keeps
 * telegram_files.accessed_at for least-recently-accessed eviction
 * (docs/design/storage-cache.md section 10). The web never writes that
 * table itself.
 *
 * Served ids collect in a process-wide set and are flushed every 60 seconds
 * from a timer on the main loop, in batches of at most 1000, to
 * POST /v1/files/access. Best effort: a failed flush is logged once and its
 * ids dropped, and nothing on a request path ever waits for the daemon.
 */

/*
 * Arm the flush timer if the daemon API is configured (api::init() first).
 * Call before drogon::app().run(). Without the API, record() stays a no-op.
 */
void start(void);

/* Note that file `id` was served. Cheap and non-blocking; any thread. */
void record(uint64_t id);

} /* namespace tgweb::daemon::access */

#endif /* TGLOGGERD_WEB_DAEMON_ACCESSREPORT_HPP */
