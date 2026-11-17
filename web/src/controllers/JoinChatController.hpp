// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#ifndef TGLOGGERD_WEB_CONTROLLERS_JOINCHATCONTROLLER_HPP
#define TGLOGGERD_WEB_CONTROLLERS_JOINCHATCONTROLLER_HPP

#include <drogon/HttpController.h>

namespace tgweb::controllers {

/*
 * /join-chat: make the logging account join a group or channel by invite
 * link or public username (web/docs/join-chat.md).
 *
 * The daemon does the work over its internal API; this page previews the
 * chat, asks for a confirmation and audits every join attempt. Joining acts
 * on the Telegram account itself, so every route is AuthFilter + AdminFilter
 * and both POSTs check the CSRF token. The preview is a POST too: it costs a
 * rate-limited Telegram lookup, which a cross-site page must not trigger.
 */
class JoinChatController : public drogon::HttpController<JoinChatController> {
public:
	METHOD_LIST_BEGIN
	ADD_METHOD_TO(JoinChatController::index, "/join-chat", drogon::Get,
		      "tgweb::auth::AuthFilter", "tgweb::auth::AdminFilter");
	ADD_METHOD_TO(JoinChatController::preview, "/join-chat/preview",
		      drogon::Post, "tgweb::auth::AuthFilter",
		      "tgweb::auth::AdminFilter");
	ADD_METHOD_TO(JoinChatController::join, "/join-chat/join", drogon::Post,
		      "tgweb::auth::AuthFilter", "tgweb::auth::AdminFilter");
	METHOD_LIST_END

	drogon::Task<drogon::HttpResponsePtr> index(drogon::HttpRequestPtr req);
	drogon::Task<drogon::HttpResponsePtr> preview(drogon::HttpRequestPtr req);
	drogon::Task<drogon::HttpResponsePtr> join(drogon::HttpRequestPtr req);
};

} /* namespace tgweb::controllers */

#endif /* TGLOGGERD_WEB_CONTROLLERS_JOINCHATCONTROLLER_HPP */
