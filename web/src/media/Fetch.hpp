// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD_WEB_MEDIA_FETCH_HPP
#define TGLOGGERD_WEB_MEDIA_FETCH_HPP

#include <cstdint>
#include <filesystem>
#include <optional>
#include <string>

namespace tgweb::media {

/*
 * Who may make the daemon fetch a file the store does not hold
 * (docs/design/storage-cache.md section 11, web/docs/media.md).
 *
 * This is the one place that decides it: /files/<token> asks mayFetch()
 * before calling the daemon, and the chat pages mark media `fetchable` from
 * the same answer, so a page never offers a load the file route would refuse.
 */
enum class FetchDecision {
	Allowed,	/* ask the daemon.                                   */
	Off,		/* WEB_TGLOGGERD_API is not set: nothing can fetch.  */
	NeedsSession,	/* a big file, and the request has no session.       */
};

/*
 * `size` is telegram_files.file_size (0 when unknown). A file under the
 * daemon's TG_MAX_STORE_FILE_SIZE is fetched whole into the store; a bigger
 * one is streamed chunk by chunk from TDLib's cache, which only a signed-in
 * request may do, so a leaked token can cost one small file at most. An
 * unknown size counts as small here; the caller refuses a `partial` answer
 * to an anonymous request anyway.
 */
FetchDecision mayFetch(uint64_t size, bool hasSession);

/* Whether a file of this size is stored whole by a fetch: under
 * TG_MAX_STORE_FILE_SIZE, or any file when that is 0. */
bool isSmall(uint64_t size);

/*
 * The TDLib directory the daemon serves partial files from: the absolute,
 * canonical form of <TG_DATA_DIR>/tdlib, resolved against the working
 * directory, which is the repository root (/workspace) in both containers
 * as for the daemon. Empty when TG_DATA_DIR is not set.
 */
const std::filesystem::path &tdlibDir(void);

/*
 * Resolve a path the daemon answered with and accept it only when it names a
 * regular file inside tdlibDir() after resolving "..", symlinks and the like.
 */
std::optional<std::filesystem::path> insideTdlibDir(const std::string &path);

} /* namespace tgweb::media */

#endif /* TGLOGGERD_WEB_MEDIA_FETCH_HPP */
