// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__STORAGE_PATH_HPP
#define TGLOGGERD__STORAGE_PATH_HPP

#include <string>
#include <filesystem>

namespace tgloggerd {

/*
 * Where a blob lives in the content-addressed store: 5 levels of two-hex-digit
 * directories from the first 5 octets of the digest, then <sha256>[.ext]. For
 * 0a533d97ed... that is <storage_dir>/0a/53/3d/97/ed/0a533d97ed...[.ext].
 *
 * The one definition of the layout in the daemon: storeDownloadedFile writes
 * here and the StorageJanitor unlinks here. The web builds the same path in
 * web/src/controllers/MediaController.cpp. sha256_hex must be 64 hex digits;
 * ext is lowercase without the dot, or empty.
 */
inline std::filesystem::path blobPath(const std::string &storage_dir,
				      const std::string &sha256_hex,
				      const std::string &ext)
{
	std::filesystem::path p = storage_dir;
	for (size_t i = 0; i < 5; i++)
		p /= sha256_hex.substr(i * 2, 2);
	std::string name = sha256_hex;
	if (!ext.empty())
		name += "." + ext;
	return p / name;
}

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__STORAGE_PATH_HPP */
