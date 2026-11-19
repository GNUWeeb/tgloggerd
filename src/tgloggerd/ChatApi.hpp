// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD__CHAT_API_HPP
#define TGLOGGERD__CHAT_API_HPP

#include "InternalApi.hpp"

namespace tgloggerd {

class TDLib;

/*
 * Register the chat routes of the internal API (docs/internal-api.md):
 * POST /v1/chats/resolve (preview an invite link or public username),
 * POST /v1/chats/join (make the account join it) and
 * POST /v1/users/resolve (find and store the user or bot behind a public
 * username). Curated operations: the input is parsed and normalised here
 * (ChatRef.hpp), never handed to TDLib as is. tdlib must outlive the API.
 */
void registerChatApi(InternalApi &api, TDLib &tdlib);

} /* namespace tgloggerd */

#endif /* #ifndef TGLOGGERD__CHAT_API_HPP */
