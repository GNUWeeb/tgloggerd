// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__EVENT_API_HPP
#define TGLOGGERD__EVENT_API_HPP

#include "InternalApi.hpp"
#include "EventBus.hpp"

namespace tgloggerd {

/*
 * Register GET /v1/events (docs/internal-api.md): the bus's events as
 * Server-Sent Events. bus must outlive the API.
 */
void registerEventApi(InternalApi &api, EventBus &bus);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__EVENT_API_HPP */
