// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__STORAGE_API_HPP
#define TGLOGGERD__STORAGE_API_HPP

#include "InternalApi.hpp"
#include "DB.hpp"
#include "StorageJanitor.hpp"

namespace tgloggerd {

/*
 * Register the storage routes of the internal API (docs/internal-api.md):
 * GET /v1/storage, POST /v1/storage/sweep. db and janitor must outlive
 * the API.
 */
void registerStorageApi(InternalApi &api, DB &db, StorageJanitor &janitor);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__STORAGE_API_HPP */
