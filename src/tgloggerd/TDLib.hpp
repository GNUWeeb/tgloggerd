// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#ifndef TGLOGGERD__TDLIB_HPP
#define TGLOGGERD__TDLIB_HPP

namespace tgloggerd {

/*
 * tgloggerd::TDLib is a wrapper class for TDLib.
 *
 * Since TDLib contains very heavy header files, keep tgloggerd
 * compilation time low by not including TDLib header files in
 * other tgloggerd files. Expose only used functions in
 * tgloggerd::TDLib class.
 *
 * Only TDLib.cpp is allowed to include TDLib header files.
 * Other tgloggerd files should include TDLib.hpp if they need to
 * use TDLib.
 */
class TDLib {
public:
	TDLib(int api_id, const char *api_hash, const char *data_dir);
	~TDLib(void);
private:
};

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__TDLIB_HPP */
