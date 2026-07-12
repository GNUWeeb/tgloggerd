// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__TGLOGGERD_HPP
#define TGLOGGERD__TGLOGGERD_HPP

#include <cstdint>
#include "helpers/log.h"

namespace tgloggerd {

class TgLoggerd {
public:
	TgLoggerd(uint32_t api_id, const char *api_hash, const char *data_dir) noexcept;
	~TgLoggerd(void);
	int start(void);
	int stop(void);
	void setLogger(log_hd_t *h) noexcept;
	void setLoggerLevel(int8_t log_level) noexcept;

private:
	inline int initDataDir(void);
	inline int initDataDirC(const char *dir);

	uint32_t api_id_;
	char api_hash_[64];
	char data_dir_[512];
	log_hd_t *l_ = nullptr;
};

} /* namespace tgloggerd */
#endif /* #ifndef TGLOGGERD__TGLOGGERD_HPP */
