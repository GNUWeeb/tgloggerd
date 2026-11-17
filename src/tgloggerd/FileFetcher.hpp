// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__FILE_FETCHER_HPP
#define TGLOGGERD__FILE_FETCHER_HPP

#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <functional>
#include <mutex>
#include <optional>
#include <string>
#include <thread>
#include <unordered_map>

#include "DB.hpp"
#include "TDLib.hpp"
#include "FetchState.hpp"
#include "helpers/log.h"

namespace tgloggerd {

/*
 * Fetches a telegram_files row's bytes from Telegram when the web asks for
 * them (POST /v1/files/{id}/fetch, storage-cache.md section 8):
 *
 *  - a small file (under TG_MAX_STORE_FILE_SIZE) is downloaded whole and
 *    stored the way ingest stores it, so the web serves it from the store;
 *  - a big file is downloaded a range at a time into TDLib's cache, and
 *    the answer says where on the shared mount the bytes are. It is never
 *    copied into the store; TDLib's copy is deleted once it has not been
 *    asked for in TG_FETCH_IDLE_SECS.
 *
 * fetch() runs on the internal API's handler threads and blocks on
 * TDLib::request; it throws InternalApi::Error for every error answer.
 */
class FileFetcher {
public:
	struct Config {
		size_t			max_active = 4;
		uint64_t		chunk = 16ull << 20;
		std::chrono::seconds	timeout{30};
		std::chrono::seconds	idle{900};
		/* TG_MAX_STORE_FILE_SIZE; 0 = every file is small. */
		uint64_t		max_store_file_size = 0;
		/* TG_PRUNE_TDLIB_FILES: delete TDLib's copy once stored. */
		bool			prune_stored = true;
		/* Absolute TDLib directory; every partial path is under it. */
		std::string		tdlib_dir;

		/*
		 * The TG_FETCH_* settings from the environment, defaults for
		 * unset or invalid ones; the other fields stay as they are.
		 */
		void loadEnv(void);
	};

	/*
	 * Store a downloaded file exactly as ingest does
	 * (TgLoggerd::storeDownloadedFile): (local path, tg_file_id,
	 * tg_unique_id, size, file_type, original name) -> the id of the row
	 * now holding the bytes, or nullopt when it could not be stored.
	 */
	using StoreFn = std::function<std::optional<uint64_t>(
		const std::string &, const std::string &, const std::string &,
		int64_t, const std::string &, const std::string &)>;
	/* Whether the store holds the blob (sha256 hex, extension). */
	using BlobFn = std::function<bool(const std::string &,
					  const std::string &)>;
	/*
	 * Whether an ingest store job still has to read TDLib's copy of
	 * local file @fid (TgLoggerd's StoreHolds); ingest prunes it then.
	 */
	using HeldFn = std::function<bool(int32_t)>;

	struct Result {
		bool		stored = false;
		uint64_t	id = 0;		/* stored: the row with the bytes */
		/* partial: */
		std::string	path;
		uint64_t	offset = 0;
		uint64_t	available = 0;
		uint64_t	size = 0;
	};

	FileFetcher(DB &db, TDLib &tdlib, log_hd_t *l, Config cfg,
		    StoreFn store, BlobFn blob_present, HeldFn ingest_held);
	~FileFetcher(void);

	FileFetcher(const FileFetcher &) = delete;
	FileFetcher &operator=(const FileFetcher &) = delete;

	/* Start the thread that deletes idle partial files. */
	void start(void);
	/*
	 * Stop that thread. Idempotent. Must run before the TDLib object
	 * goes away and after the API has stopped calling fetch().
	 */
	void stop(void);

	/*
	 * Make @length bytes from @offset of file @id local (a whole small
	 * file regardless). @length 0 means one chunk.
	 */
	Result fetch(uint64_t id, uint64_t offset, uint64_t length);

	/* Whether @id was found gone from Telegram within the last hour. */
	bool isGone(uint64_t id);

private:
	std::optional<Result> storedCopy(const FetchRow &row);
	TDLib::TdObject resolveKnown(const FetchRow &row,
				     fetch::Clock::time_point deadline);
	TDLib::TdObject resolveFresh(const FetchRow &row,
				     fetch::Clock::time_point deadline);
	Result fetchFile(const FetchRow &row, const TDLib::TdObject &file,
			 uint64_t offset, uint64_t length,
			 fetch::Clock::time_point deadline);
	std::optional<Result> fetchWhole(const FetchRow &row, int32_t fid,
					 int64_t size,
					 const std::string &unique_id,
					 fetch::Clock::time_point deadline,
					 std::optional<fetch::Clock::time_point> &prune_at);
	Result fetchRange(const FetchRow &row, int32_t fid, uint64_t size,
			  uint64_t offset, uint64_t length,
			  fetch::Clock::time_point deadline);
	TDLib::TdObject call(TDLib::TdFunction f,
			     fetch::Clock::time_point deadline);
	uint64_t prefixAt(int32_t fid, uint64_t offset,
			  fetch::Clock::time_point deadline);
	void rememberFid(uint64_t id, int32_t fid);
	void forgetFid(uint64_t id);
	void reaperLoop(void);
	bool ingestNeeds(int32_t fid) const;

	DB			&db_;
	TDLib			&tdlib_;
	log_hd_t		*l_;
	Config			cfg_;
	StoreFn			store_;
	BlobFn			blob_present_;
	HeldFn			ingest_held_;

	fetch::GoneCache	gone_;
	fetch::ActiveLimit	limit_;
	fetch::LeaseTable	leases_;

	/* telegram_files.id -> TDLib file id, valid for this TDLib session. */
	std::mutex				fids_mtx_;
	std::unordered_map<uint64_t, int32_t>	fids_;

	std::mutex		reaper_mtx_;
	std::condition_variable	reaper_cv_;
	bool			reaper_stop_ = false;
	std::thread		reaper_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__FILE_FETCHER_HPP */
