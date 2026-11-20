// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__PHOTO_API_HPP
#define TGLOGGERD__PHOTO_API_HPP

#include "InternalApi.hpp"

namespace tgloggerd {

class TDLib;
class DB;

/*
 * Register the photo list routes of the internal API (docs/internal-api.md):
 * GET /v1/users/{user_id}/photos and GET /v1/chats/{chat_id}/photos. Each
 * lists the photos as Telegram has them, records the file rows and the
 * history entries the archive lacks, and is cached for ten minutes. tdlib
 * and db must outlive the API.
 */
void registerPhotoApi(InternalApi &api, TDLib &tdlib, DB &db);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__PHOTO_API_HPP */
