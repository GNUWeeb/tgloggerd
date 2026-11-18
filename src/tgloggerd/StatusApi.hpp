// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__STATUS_API_HPP
#define TGLOGGERD__STATUS_API_HPP

#include "InternalApi.hpp"
#include "TDLib.hpp"

#include <functional>

namespace tgloggerd {

/*
 * Register GET /v1/status (docs/internal-api.md): the TDLib client's
 * state and since when. status is called on a handler thread for every
 * request, so it must be cheap and thread-safe (TDLib::status() is); it
 * must outlive the API.
 */
void registerStatusApi(InternalApi &api,
		       std::function<TDLib::Status()> status);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__STATUS_API_HPP */
