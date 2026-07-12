// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/helpers/common.h>
#include <tgloggerd/TgLoggerd.hpp>
#include <stdexcept>
#include <cstring>
#include <cstdio>

namespace tgloggerd {

TgLoggerd::TgLoggerd(uint32_t api_id, const char *api_hash, const char *data_dir) noexcept
{
	this->api_id_ = api_id;
	snprintf(this->api_hash_, sizeof(this->api_hash_), "%s", api_hash);
	snprintf(this->data_dir_, sizeof(this->data_dir_), "%s", data_dir);
}

TgLoggerd::~TgLoggerd(void)
{
	pr_debug(l_, "Destroying TgLoggerd...");
	pr_free(l_);
}

inline int TgLoggerd::initDataDirC(const char *dir)
{
	char path[sizeof(this->data_dir_) + 256];
	int r;

	snprintf(path, sizeof(path), "%s/%s", this->data_dir_, dir);
	pr_debug(l_, "Initializing data directory: %s", path);
	r = mkdir_recursive(path, 0700);
	if (r < 0) {
		if (r == -EEXIST) {
			pr_debug(l_, "Data directory already exists: %s", path);
			return 0;
		}

		pr_error(l_, "Failed to create data directory: %s, error: %s",
			 path, strerror(-r));
		return r;
	} else {
		pr_debug(l_, "Data directory created: %s", path);
		return 0;
	}
}

inline int TgLoggerd::initDataDir(void)
{
	int r;

	r = initDataDirC("logs");
	if (r < 0)
		return r;

	r = initDataDirC("tdlib");
	if (r < 0)
		return r;

	return 0;
}

int TgLoggerd::start(void)
{
	if (initDataDir())
		return -1;

	pr_info(l_, "Starting tgloggerd...");
	pr_debug(l_, "api_id: %u", this->api_id_);
	pr_debug(l_, "api_hash: %s", this->api_hash_);
	pr_debug(l_, "data_dir: %s", this->data_dir_);
	return 0;
}

int TgLoggerd::stop(void)
{
	pr_info(l_, "Stopping tgloggerd...");
	return 0;
}

void TgLoggerd::setLogger(log_hd_t *h) noexcept
{
	l_ = h;
}

void TgLoggerd::setLoggerLevel(int8_t log_level) noexcept
{
	pr_set_log_level(l_, log_level);
}

} /* namespace tgloggerd */
