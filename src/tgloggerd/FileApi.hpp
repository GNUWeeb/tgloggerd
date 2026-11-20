// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__FILE_API_HPP
#define TGLOGGERD__FILE_API_HPP

#include "InternalApi.hpp"
#include "DB.hpp"
#include "FileFetcher.hpp"

namespace tgloggerd {

/* Most ids one POST /v1/files/access may carry. */
constexpr size_t kMaxAccessBatch = 1000;

/*
 * Register the file routes of the internal API (docs/internal-api.md):
 * GET /v1/files/{id}, POST /v1/files/access and, with a fetcher,
 * POST /v1/files/{id}/fetch. db and fetcher must outlive the API.
 */
void registerFileApi(InternalApi &api, DB &db, FileFetcher *fetcher);

/* Parse a path's file id: decimal, 1 .. 2^64-1; throws a 400 otherwise. */
uint64_t parseFileId(const std::string &s);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__FILE_API_HPP */
