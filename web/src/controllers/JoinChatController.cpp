// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "controllers/JoinChatController.hpp"

#include "auth/Csrf.hpp"
#include "auth/Session.hpp"
#include "controllers/Common.hpp"
#include "daemon/Api.hpp"
#include "dao/Audit.hpp"
#include "views/Render.hpp"

#include <exception>
#include <string>

namespace tgweb::controllers {

namespace {

drogon::HttpResponsePtr jsonResp(const nlohmann::json &j,
				 drogon::HttpStatusCode code = drogon::k200OK)
{
	auto resp = drogon::HttpResponse::newHttpResponse();
	resp->setStatusCode(code);
	resp->setContentTypeCode(drogon::CT_APPLICATION_JSON);
	resp->setBody(j.dump());
	return resp;
}

drogon::HttpResponsePtr jsonError(const std::string &msg,
				  drogon::HttpStatusCode code = drogon::k400BadRequest)
{
	return jsonResp({ { "ok", false }, { "error", msg } }, code);
}

std::string str(const nlohmann::json &j, const char *key)
{
	if (j.is_object() && j.contains(key) && j[key].is_string())
		return j[key].get<std::string>();
	return std::string();
}

/* Cut s to at most n codepoints (web_audit.detail is VARCHAR(255) utf8mb4),
 * never splitting a multibyte character. */
std::string clipChars(const std::string &s, size_t n)
{
	size_t chars = 0, i = 0;
	while (i < s.size() && chars < n) {
		const unsigned char b = (unsigned char)s[i];
		size_t len = b < 0x80 ? 1 :
			     (b >> 5) == 0x06 ? 2 :
			     (b >> 4) == 0x0E ? 3 :
			     (b >> 3) == 0x1E ? 4 : 1;
		if (i + len > s.size())
			break;
		i += len;
		chars++;
	}
	return s.substr(0, i);
}

/* What the admin reads for a daemon outcome (docs/internal-api.md). */
std::string outcomeText(const nlohmann::json &b)
{
	const std::string o = str(b, "outcome");
	const std::string m = str(b, "message");
	if (o == "ok")
		return "";
	if (o == "joined")
		return "Joined. The chat is now logged, media included.";
	if (o == "already_member")
		return "The account is already a member of this chat.";
	if (o == "request_sent")
		return "Join request sent. An administrator of the chat has to "
		       "approve it; the chat is logged once they do.";
	if (o == "approval_required")
		return "The chat's guard bot must approve the join. Finish it "
		       "in a Telegram client logged in as the account.";
	if (o == "declined")
		return "The chat's guard bot declined the join.";
	if (o == "not_a_group")
		return "That username belongs to a user or a bot, not a group "
		       "or channel.";
	if (o == "not_found")
		return "No public chat has that username.";
	if (o == "link_invalid")
		return "The invite link is invalid, expired or revoked.";
	if (o == "flood_wait") {
		int s = b.value("retry_after", 0);
		if (s <= 0)
			return "Telegram is rate-limiting the account. Wait a "
			       "while before trying again.";
		std::string wait = s < 120 ? std::to_string(s) + " seconds"
			: s < 7200 ? std::to_string((s + 59) / 60) + " minutes"
				   : std::to_string((s + 3599) / 3600) + " hours";
		return "Telegram is rate-limiting the account. Try again in " +
		       wait + ".";
	}
	if (o == "banned")
		return "The account is banned from this chat, or the chat is "
		       "private and not accessible to it.";
	if (o == "too_many_chats")
		return "The account is in too many chats to join another.";
	return "Telegram refused: " + (m.empty() ? std::string("unknown error") : m);
}

/*
 * A daemon answer that is not a 200 with an outcome, as a message for the
 * admin. `joining` adds the caveat that a timed-out join may have happened.
 */
std::string daemonFailure(const daemon::api::Reply &r, bool joining)
{
	if (!daemon::api::enabled())
		return "The logger daemon's API is not configured "
		       "(WEB_TGLOGGERD_API), so chats cannot be joined from "
		       "here.";
	if (!r.sent)
		return joining
			? "The logger daemon did not answer. The join may still "
			  "have happened: preview the chat again to check."
			: "The logger daemon is not reachable. Try again later.";
	const std::string m = str(r.body, "message");
	switch (r.status) {
	case 400:
		return m.empty() ? "The daemon refused that input." : "Not accepted: " + m + ".";
	case 401:
		return "The daemon refused the web's key: WEB_APP_KEY differs "
		       "between the two.";
	case 503:
		return "The Telegram client is not ready (" + m +
		       "). Try again in a moment.";
	case 504:
		return joining
			? "Telegram did not answer in time. The join may still "
			  "have happened: preview the chat again to check."
			: "Telegram did not answer in time. Try again.";
	default:
		return "The daemon answered HTTP " + std::to_string(r.status) +
		       (m.empty() ? std::string(".") : ": " + m);
	}
}

/* The ref the admin typed: trimmed, bounded. The daemon does the real
 * parsing; this only keeps junk off the wire. */
std::string formRef(const drogon::HttpRequestPtr &req)
{
	std::string ref = req->getParameter("ref");
	const char *ws = " \t\r\n";
	size_t a = ref.find_first_not_of(ws);
	if (a == std::string::npos)
		return std::string();
	size_t b = ref.find_last_not_of(ws);
	return ref.substr(a, b - a + 1);
}

nlohmann::json chatOf(const nlohmann::json &b)
{
	if (b.is_object() && b.contains("chat") && b["chat"].is_object())
		return b["chat"];
	return nullptr;
}

} /* namespace */

drogon::Task<drogon::HttpResponsePtr>
JoinChatController::index(drogon::HttpRequestPtr req)
{
	nlohmann::json data = pageBase(req);
	data["title"] = "Join a chat";
	data["api_enabled"] = daemon::api::enabled();

	data["attempts"] = nlohmann::json::array();
	data["db_error"] = "";
	try {
		auto app = drogon::app().getDbClient("app");
		auto r = co_await app->execSqlCoro(
			"SELECT a.created_at, a.detail, "
			"COALESCE(u.username, '') AS username "
			"FROM web_audit a LEFT JOIN web_users u "
			"ON u.id = a.web_user_id "
			"WHERE a.action = 'chat_join' "
			"ORDER BY a.id DESC LIMIT 20");
		for (const auto &row : r) {
			data["attempts"].push_back({
				{ "created_at", views::Render::esc(
					row["created_at"].as<std::string>()) },
				{ "detail", views::Render::esc(
					row["detail"].isNull() ? std::string()
						: row["detail"].as<std::string>()) },
				{ "username", views::Render::esc(
					row["username"].as<std::string>()) },
			});
		}
	} catch (const std::exception &e) {
		data["db_error"] = views::Render::esc(e.what());
	}
	co_return htmlPage(views::Render::page("join_chat.html", data));
}

drogon::Task<drogon::HttpResponsePtr>
JoinChatController::preview(drogon::HttpRequestPtr req)
{
	if (!auth::csrf::checkSession(req, req->getParameter("csrf")))
		co_return jsonError("Your session expired. Please reload.",
				    drogon::k403Forbidden);

	const std::string ref = formRef(req);
	if (ref.empty())
		co_return jsonError("Paste an invite link or a @username.");
	if (ref.size() > 256)
		co_return jsonError("That is too long for an invite link or a "
				    "username.");

	auto r = co_await daemon::api::resolveChat(ref);
	if (!r.ok() || !r.body.is_object() || !r.body.contains("outcome"))
		co_return jsonError(daemonFailure(r, false),
				    r.sent && r.status == 400
					    ? drogon::k400BadRequest
					    : drogon::k502BadGateway);

	co_return jsonResp({
		{ "ok", true },
		{ "outcome", str(r.body, "outcome") },
		{ "message", outcomeText(r.body) },
		{ "ref", str(r.body, "ref") },
		{ "kind", str(r.body, "kind") },
		{ "chat", chatOf(r.body) },
	});
}

drogon::Task<drogon::HttpResponsePtr>
JoinChatController::join(drogon::HttpRequestPtr req)
{
	if (!auth::csrf::checkSession(req, req->getParameter("csrf")))
		co_return jsonError("Your session expired. Please reload.",
				    drogon::k403Forbidden);

	auto s = auth::session::current(req);
	if (!s)
		co_return jsonError("Not signed in.", drogon::k403Forbidden);

	const std::string ref = formRef(req);
	if (ref.empty() || ref.size() > 256)
		co_return jsonError("Paste an invite link or a @username.");

	auto r = co_await daemon::api::joinChat(ref);
	const bool answered =
		r.ok() && r.body.is_object() && r.body.contains("outcome");

	/*
	 * One row per attempt, whatever came of it: what was asked (the
	 * daemon's normalised ref when it answered) and the outcome, or why
	 * the daemon gave none.
	 */
	std::string outcome, detail;
	nlohmann::json chat = answered ? chatOf(r.body) : nlohmann::json(nullptr);
	if (answered) {
		outcome = str(r.body, "outcome");
		detail = outcome + " " + str(r.body, "ref");
		if (chat.is_object() && chat.contains("title") &&
		    chat["title"].is_string())
			detail += " \"" + chat["title"].get<std::string>() + "\"";
		if (outcome == "flood_wait")
			detail += " retry_after=" +
				  std::to_string(r.body.value("retry_after", 0));
	} else {
		outcome = !daemon::api::enabled() ? "api_off"
			  : !r.sent ? "no_answer"
				    : "http_" + std::to_string(r.status);
		detail = outcome + " " + ref;
	}
	try {
		co_await dao::audit::log(drogon::app().getDbClient("app"), s->uid,
					 "chat_join", req->getPeerAddr().toIp(),
					 clipChars(detail, 255));
	} catch (const std::exception &e) {
		/* The join already happened or not; losing the row must not
		 * hide its outcome from the admin. */
		LOG_ERROR << "join-chat: audit insert failed: " << e.what();
	}

	if (!answered)
		co_return jsonError(daemonFailure(r, true),
				    r.sent && r.status == 400
					    ? drogon::k400BadRequest
					    : drogon::k502BadGateway);

	nlohmann::json out = {
		{ "ok", true },
		{ "outcome", outcome },
		{ "message", outcomeText(r.body) },
		{ "ref", str(r.body, "ref") },
		{ "chat", chat },
	};
	if (outcome == "flood_wait")
		out["retry_after"] = r.body.value("retry_after", 0);
	co_return jsonResp(out);
}

} /* namespace tgweb::controllers */
