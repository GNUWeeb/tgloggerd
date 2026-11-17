// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__FILE_INDEX_HPP
#define TGLOGGERD__FILE_INDEX_HPP

#include <mutex>
#include <optional>
#include <string>
#include <unordered_map>
#include <utility>
#include "models/File.hpp"

namespace tgloggerd {

/*
 * The in-memory index of recorded files (telegram_files rows), which lets
 * the TDLib loop thread decide without a DB round-trip that a message's
 * media, link preview image or a user's or group's photo is already
 * recorded and can be linked instead of downloaded again.
 *
 * Keyed twice: by remote id (tg_file_id) and by TDLib's unique id
 * (tg_unique_id). The remote id of the same file often differs between
 * messages and between updates of a photo, the unique id does not, so a
 * lookup by remote id alone misses files that are already stored.
 *
 * Thread-safe: the loop thread looks files up, the store and metadata
 * workers add them.
 */
class FileIndex {
public:
	using Map = std::unordered_map<std::string, models::KnownFile>;

	/* Replace the contents (startup seeding from the DB). */
	void load(Map by_file_id, Map by_unique_id)
	{
		std::lock_guard<std::mutex> lk(mtx_);
		by_file_id_ = std::move(by_file_id);
		by_unique_id_ = std::move(by_unique_id);
	}

	/*
	 * The row recorded for a file with remote id @tg_file_id and unique
	 * id @tg_unique_id (either may be empty). A row holding the bytes is
	 * preferred: by remote id, else by unique id (stored or not), else a
	 * metadata-only row by remote id.
	 */
	std::optional<models::KnownFile>
	find(const std::string &tg_file_id,
	     const std::string &tg_unique_id) const
	{
		std::lock_guard<std::mutex> lk(mtx_);
		auto it = by_file_id_.end();
		if (!tg_file_id.empty())
			it = by_file_id_.find(tg_file_id);
		if (it != by_file_id_.end() && !it->second.meta_only)
			return it->second;
		if (!tg_unique_id.empty()) {
			auto uit = by_unique_id_.find(tg_unique_id);
			if (uit != by_unique_id_.end())
				return uit->second;
		}
		if (it != by_file_id_.end())
			return it->second;
		return std::nullopt;
	}

	/* Record that the file is now row @k (a store or a metadata record). */
	void put(const std::string &tg_file_id, const std::string &tg_unique_id,
		 const models::KnownFile &k)
	{
		std::lock_guard<std::mutex> lk(mtx_);
		if (!tg_file_id.empty())
			by_file_id_[tg_file_id] = k;
		if (!tg_unique_id.empty())
			by_unique_id_[tg_unique_id] = k;
	}

	std::pair<size_t, size_t> size(void) const
	{
		std::lock_guard<std::mutex> lk(mtx_);
		return { by_file_id_.size(), by_unique_id_.size() };
	}

private:
	mutable std::mutex	mtx_;
	Map			by_file_id_;
	Map			by_unique_id_;
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__FILE_INDEX_HPP */
