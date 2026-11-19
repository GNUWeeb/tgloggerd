// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ChatApi.hpp"

#include "ChatRef.hpp"
#include "DB.hpp"
#include "TDLib.hpp"

#include <td/telegram/td_api.h>

#include <chrono>
#include <ctime>
#include <map>
#include <mutex>
#include <optional>
#include <string>
#include <tuple>

namespace tgloggerd {

namespace {

namespace td_api = td::td_api;
using namespace std::chrono_literals;

/*
 * Per TDLib request. A preview is two or three requests and a join one more,
 * so a whole call stays well under a minute; the web waits a little longer.
 */
constexpr std::chrono::milliseconds kRequestTimeout = 15s;

/* A td_api::error answer, carried out of the helpers below. */
struct TdFailure {
	int32_t		code;
	std::string	message;
};

/*
 * Send f and return the answer as T, or throw TdFailure for an error (or an
 * answer of an unexpected type, which would be a TDLib API change).
 */
TDLib::TdObject callAny(TDLib &t, td_api::object_ptr<td_api::Function> f)
{
	TDLib::TdObject obj = t.request(std::move(f), kRequestTimeout);
	if (!obj)
		throw TdFailure{ 500, "empty answer from TDLib" };
	if (obj->get_id() == td_api::error::ID) {
		auto e = td::move_tl_object_as<td_api::error>(obj);
		throw TdFailure{ e->code_, e->message_ };
	}
	return obj;
}

template <class T>
td_api::object_ptr<T> call(TDLib &t, td_api::object_ptr<td_api::Function> f)
{
	TDLib::TdObject obj = callAny(t, std::move(f));
	if (obj->get_id() != T::ID)
		throw TdFailure{ 500, "unexpected answer type from TDLib" };
	return td::move_tl_object_as<T>(obj);
}

/* What a preview tells the admin about the chat. */
struct ChatInfo {
	int64_t		chat_id = 0;	/* 0: not known before joining */
	std::string	title;
	std::string	type;		/* basic_group, supergroup, channel, private */
	int32_t		member_count = 0;
	bool		is_member = false;
	bool		banned = false;
	bool		needs_approval = false;
	bool		paid = false;
	std::string	username;	/* public username, if any */

	bool joinable(void) const { return type != "private"; }

	nlohmann::json toJson(void) const
	{
		return {
			{ "chat_id", chat_id ? nlohmann::json(chat_id)
					     : nlohmann::json(nullptr) },
			{ "title", title },
			{ "type", type },
			{ "member_count", member_count },
			{ "is_member", is_member },
			{ "banned", banned },
			{ "needs_approval", needs_approval },
			{ "paid", paid },
			{ "username", username.empty() ? nlohmann::json(nullptr)
						       : nlohmann::json(username) },
		};
	}
};

/* The account's membership from its status in the group (as TDLib.cpp's
 * membership_from_status, plus the ban). */
void applyStatus(ChatInfo &ci, const td_api::ChatMemberStatus *s)
{
	if (!s)
		return;
	switch (s->get_id()) {
	case td_api::chatMemberStatusCreator::ID:
	case td_api::chatMemberStatusAdministrator::ID:
	case td_api::chatMemberStatusMember::ID:
		ci.is_member = true;
		break;
	case td_api::chatMemberStatusRestricted::ID:
		ci.is_member = static_cast<const td_api::chatMemberStatusRestricted &>(
				       *s).is_member_;
		break;
	case td_api::chatMemberStatusBanned::ID:
		ci.banned = true;
		break;
	default:
		break;
	}
}

/*
 * Fill ci from a chat the account can see: its type, and for a group its
 * member count, the account's status and whether joining needs approval.
 */
void describeChat(TDLib &t, ChatInfo &ci, const td_api::chat &c)
{
	ci.chat_id = c.id_;
	ci.title = c.title_;
	if (!c.type_) {
		ci.type = "private";
		return;
	}

	switch (c.type_->get_id()) {
	case td_api::chatTypeSupergroup::ID: {
		const auto &st = static_cast<const td_api::chatTypeSupergroup &>(
			*c.type_);
		ci.type = st.is_channel_ ? "channel" : "supergroup";
		auto sg = call<td_api::supergroup>(
			t, td_api::make_object<td_api::getSupergroup>(
				   st.supergroup_id_));
		applyStatus(ci, sg->status_.get());
		ci.member_count = sg->member_count_;
		ci.needs_approval = sg->join_by_request_;
		if (sg->usernames_ && !sg->usernames_->active_usernames_.empty())
			ci.username = sg->usernames_->active_usernames_[0];
		if (ci.member_count == 0) {
			/* The count is often 0 until the full info is loaded;
			 * worth one more request, not worth failing for. */
			try {
				auto fi = call<td_api::supergroupFullInfo>(
					t, td_api::make_object<
						   td_api::getSupergroupFullInfo>(
						   st.supergroup_id_));
				ci.member_count = fi->member_count_;
			} catch (const TdFailure &) {
			}
		}
		break;
	}
	case td_api::chatTypeBasicGroup::ID: {
		const auto &bt = static_cast<const td_api::chatTypeBasicGroup &>(
			*c.type_);
		ci.type = "basic_group";
		auto bg = call<td_api::basicGroup>(
			t, td_api::make_object<td_api::getBasicGroup>(
				   bt.basic_group_id_));
		applyStatus(ci, bg->status_.get());
		ci.member_count = bg->member_count_;
		break;
	}
	default:
		/* A user, a bot or a secret chat: nothing to join. */
		ci.type = "private";
		break;
	}
}

ChatInfo previewInvite(TDLib &t, const ChatRef &ref)
{
	auto info = call<td_api::chatInviteLinkInfo>(
		t, td_api::make_object<td_api::checkChatInviteLink>(
			   ref.canonical()));

	ChatInfo ci;
	ci.title = info->title_;
	ci.member_count = info->member_count_;
	ci.needs_approval = info->creates_join_request_;
	ci.paid = info->subscription_info_ != nullptr;
	if (info->type_) {
		switch (info->type_->get_id()) {
		case td_api::inviteLinkChatTypeBasicGroup::ID:
			ci.type = "basic_group";
			break;
		case td_api::inviteLinkChatTypeChannel::ID:
			ci.type = "channel";
			break;
		default:
			ci.type = "supergroup";
			break;
		}
	} else {
		ci.type = "supergroup";
	}

	/*
	 * A chat_id means the account can already see the chat: it is a
	 * member, the chat is public, or the link grants a preview. Its
	 * status there tells which.
	 */
	if (info->chat_id_ != 0) {
		auto c = call<td_api::chat>(
			t, td_api::make_object<td_api::getChat>(info->chat_id_));
		int32_t count = ci.member_count;
		bool approval = ci.needs_approval;
		describeChat(t, ci, *c);
		if (ci.member_count == 0)
			ci.member_count = count;
		/* The link decides whether it creates a join request, not
		 * the chat's own setting. */
		ci.needs_approval = approval;
		ci.title = c->title_.empty() ? info->title_ : c->title_;
	}
	return ci;
}

ChatInfo previewUsername(TDLib &t, const ChatRef &ref)
{
	auto c = call<td_api::chat>(
		t, td_api::make_object<td_api::searchPublicChat>(ref.value));
	ChatInfo ci;
	describeChat(t, ci, *c);
	return ci;
}

ChatInfo preview(TDLib &t, const ChatRef &ref)
{
	return ref.kind == ChatRef::Kind::Invite ? previewInvite(t, ref)
						 : previewUsername(t, ref);
}

/* The body's "ref", parsed; a 400 when missing or not acceptable. */
ChatRef requireRef(const InternalApi::Request &req)
{
	const auto &b = req.body;
	if (!b.is_object() || !b.contains("ref") || !b["ref"].is_string())
		throw InternalApi::Error::badRequest(
			"want {\"ref\": \"<invite link or @username>\"}");
	auto ref = parseChatRef(b["ref"].get<std::string>());
	if (!ref)
		throw InternalApi::Error::badRequest(
			"not an invite link (https://t.me/+..., "
			"https://t.me/joinchat/...) or a public username "
			"(@name, https://t.me/name)");
	return *ref;
}

/*
 * The answer for a failed TDLib request: TDLib not ready or too slow is an
 * API error (503, 504), anything Telegram said is an outcome.
 */
InternalApi::Response failure(const ChatRef &ref, const TdFailure &f,
			      const std::optional<ChatInfo> &ci)
{
	ChatOutcome o = classifyChatError(f.code, f.message);
	if (o.outcome == "unavailable")
		throw InternalApi::Error::unavailable(
			"TDLib is not ready: " + f.message);
	if (o.outcome == "timeout")
		throw InternalApi::Error::timeout(
			"Telegram did not answer in time");

	InternalApi::Response r;
	r.body = {
		{ "outcome", o.outcome },
		{ "message", o.message },
		{ "ref", ref.canonical() },
		{ "kind", ref.kindName() },
		{ "chat", ci ? ci->toJson() : nlohmann::json(nullptr) },
	};
	if (o.outcome == "flood_wait")
		r.body["retry_after"] = o.retry_after;
	return r;
}

InternalApi::Response answer(const ChatRef &ref, const std::string &outcome,
			     const std::string &message, const ChatInfo &ci)
{
	InternalApi::Response r;
	r.body = {
		{ "outcome", outcome },
		{ "message", message },
		{ "ref", ref.canonical() },
		{ "kind", ref.kindName() },
		{ "chat", ci.toJson() },
	};
	return r;
}

/* The body's "username", parsed with the chat username rules; a 400 when
 * missing, malformed, or an invite link. */
ChatRef requireUsername(const InternalApi::Request &req)
{
	const auto &b = req.body;
	if (!b.is_object() || !b.contains("username") ||
	    !b["username"].is_string())
		throw InternalApi::Error::badRequest(
			"want {\"username\": \"<@username>\"}");
	auto ref = parseChatRef(b["username"].get<std::string>());
	if (!ref || ref->kind != ChatRef::Kind::Username)
		throw InternalApi::Error::badRequest(
			"not a public username (@name, name, "
			"https://t.me/name)");
	return *ref;
}

/*
 * How long an answer to POST /v1/users/resolve is reused for the same
 * username without asking Telegram again. Only answers that will not
 * change soon are kept: ok, not_found, not_a_user.
 */
constexpr std::chrono::seconds kUserResolveCacheTtl = 10min;

struct CachedUserAnswer {
	std::chrono::steady_clock::time_point	at;
	nlohmann::json				body;
};

/* The username as a cache key: Telegram usernames are case-insensitive. */
std::string usernameKey(const std::string &name)
{
	std::string k = name;
	for (char &ch : k)
		if (ch >= 'A' && ch <= 'Z')
			ch = (char)(ch - 'A' + 'a');
	return k;
}

nlohmann::json userAnswer(const std::string &outcome, const std::string &message,
			  nlohmann::json user)
{
	return {
		{ "outcome", outcome },
		{ "message", message },
		{ "user", std::move(user) },
	};
}

/*
 * POST /v1/users/resolve: find the user or bot behind a public username,
 * read-only on the account, and have the daemon store it as it stores a
 * user it sees in a chat.
 */
InternalApi::Response resolveUser(TDLib &t, const InternalApi::Request &req)
{
	static std::mutex cache_mtx;
	static std::map<std::string, CachedUserAnswer> cache;

	ChatRef ref = requireUsername(req);
	const std::string key = usernameKey(ref.value);
	const auto now = std::chrono::steady_clock::now();
	{
		std::lock_guard<std::mutex> lk(cache_mtx);
		auto it = cache.find(key);
		if (it != cache.end() && now - it->second.at < kUserResolveCacheTtl) {
			InternalApi::Response r;
			r.body = it->second.body;
			return r;
		}
	}

	nlohmann::json body;
	try {
		auto c = call<td_api::chat>(
			t, td_api::make_object<td_api::searchPublicChat>(ref.value));
		if (!c->type_ ||
		    c->type_->get_id() != td_api::chatTypePrivate::ID) {
			body = userAnswer("not_a_user",
					  "that username is a group or a "
					  "channel, not a user", nullptr);
		} else {
			int64_t uid = static_cast<const td_api::chatTypePrivate &>(
				*c->type_).user_id_;
			auto u = call<td_api::user>(
				t, td_api::make_object<td_api::getUser>(uid));
			std::string username = ref.value;
			if (u->usernames_ &&
			    !u->usernames_->active_usernames_.empty())
				username = u->usernames_->active_usernames_[0];
			const bool is_bot = u->type_ &&
				u->type_->get_id() == td_api::userTypeBot::ID;
			/*
			 * Store it the way a user seen in a message is stored:
			 * the profile row, its usernames and type, the full
			 * info and the profile photo (downloaded into the
			 * store, with its history), through the msg_count
			 * refetch path on the TDLib loop.
			 */
			t.refetchUser(uid);
			body = userAnswer("ok", "", {
				{ "user_id", uid },
				{ "username", username },
				{ "is_bot", is_bot },
			});
		}
	} catch (const TdFailure &f) {
		ChatOutcome o = classifyChatError(f.code, f.message);
		if (o.outcome == "unavailable")
			throw InternalApi::Error::unavailable(
				"TDLib is not ready: " + f.message);
		if (o.outcome == "timeout")
			throw InternalApi::Error::timeout(
				"Telegram did not answer in time");
		if (o.outcome == "not_found") {
			body = userAnswer("not_found", "no such username",
					  nullptr);
		} else if (o.outcome == "flood_wait") {
			body = userAnswer("flood_wait", o.message, nullptr);
			body["retry_after"] = o.retry_after;
		} else {
			body = userAnswer("error", f.message, nullptr);
		}
	}

	const std::string outcome = body["outcome"].get<std::string>();
	if (outcome == "ok" || outcome == "not_found" || outcome == "not_a_user") {
		std::lock_guard<std::mutex> lk(cache_mtx);
		/* Drop what has expired, so the cache stays small. */
		for (auto it = cache.begin(); it != cache.end();)
			it = now - it->second.at >= kUserResolveCacheTtl
				     ? cache.erase(it) : std::next(it);
		cache[key] = CachedUserAnswer{ now, body };
	}

	InternalApi::Response r;
	r.body = std::move(body);
	return r;
}

/* A chat id from the path: a nonzero integer; a 400 otherwise. */
int64_t requireChatId(const InternalApi::Request &req)
{
	const std::string &s = req.param("chat_id");
	size_t used = 0;
	int64_t id = 0;
	try {
		id = std::stoll(s, &used);
	} catch (const std::exception &) {
		used = 0;
	}
	if (used == 0 || used != s.size() || id == 0)
		throw InternalApi::Error::badRequest("chat_id must be an integer");
	return id;
}

/* The value of parameter @name in a raw query string ("" if absent).
 * No percent-decoding: the parameters read with it are integers. */
std::string queryValue(const std::string &query, const std::string &name)
{
	size_t i = 0;
	while (i <= query.size()) {
		size_t amp = query.find('&', i);
		if (amp == std::string::npos)
			amp = query.size();
		std::string kv = query.substr(i, amp - i);
		i = amp + 1;
		size_t eq = kv.find('=');
		if (kv.substr(0, eq) == name)
			return eq == std::string::npos ? std::string()
						       : kv.substr(eq + 1);
	}
	return std::string();
}

/* A query parameter as an integer in [lo, hi], or dflt when absent. */
int64_t queryInt(const InternalApi::Request &req, const char *name,
		 int64_t dflt, int64_t lo, int64_t hi)
{
	const std::string v = queryValue(req.query, name);
	if (v.empty())
		return dflt;
	size_t used = 0;
	int64_t n = 0;
	try {
		n = std::stoll(v, &used);
	} catch (const std::exception &) {
		used = 0;
	}
	if (used == 0 || used != v.size() || n < lo || n > hi)
		throw InternalApi::Error::badRequest(
			std::string(name) + " must be an integer from " +
			std::to_string(lo) + " to " + std::to_string(hi));
	return n;
}

/* What the account is in a group, from TDLib's (local) group object. */
struct GroupSide {
	bool		is_supergroup = false;
	bool		is_channel = false;
	int64_t		group_id = 0;	/* supergroup or basic group id */
	bool		is_member = false;
	bool		is_admin = false;
};

/* getChat, then the group behind it. getChat and getSupergroup /
 * getBasicGroup answer from TDLib's own cache. Throws TdFailure; a chat
 * that is not a group is a 404. */
GroupSide groupSide(TDLib &t, int64_t chat_id)
{
	auto c = call<td_api::chat>(t, td_api::make_object<td_api::getChat>(chat_id));
	GroupSide g;
	const td_api::ChatMemberStatus *status = nullptr;
	td_api::object_ptr<td_api::supergroup> sg;
	td_api::object_ptr<td_api::basicGroup> bg;
	if (c->type_ && c->type_->get_id() == td_api::chatTypeSupergroup::ID) {
		auto &st = static_cast<const td_api::chatTypeSupergroup &>(*c->type_);
		g.is_supergroup = true;
		g.is_channel = st.is_channel_;
		g.group_id = st.supergroup_id_;
		sg = call<td_api::supergroup>(
			t, td_api::make_object<td_api::getSupergroup>(g.group_id));
		status = sg->status_.get();
	} else if (c->type_ && c->type_->get_id() == td_api::chatTypeBasicGroup::ID) {
		g.group_id = static_cast<const td_api::chatTypeBasicGroup &>(
			*c->type_).basic_group_id_;
		bg = call<td_api::basicGroup>(
			t, td_api::make_object<td_api::getBasicGroup>(g.group_id));
		status = bg->status_.get();
	} else {
		throw InternalApi::Error::notFound("not a group or channel");
	}
	ChatInfo ci;
	applyStatus(ci, status);
	g.is_member = ci.is_member;
	g.is_admin = status &&
		(status->get_id() == td_api::chatMemberStatusCreator::ID ||
		 status->get_id() == td_api::chatMemberStatusAdministrator::ID);
	return g;
}

/* Throw 503 or 504 for TDLib not ready or too slow; return for anything
 * Telegram said, which the caller answers itself. */
void throwIfApiFailure(const TdFailure &f)
{
	ChatOutcome o = classifyChatError(f.code, f.message);
	if (o.outcome == "unavailable")
		throw InternalApi::Error::unavailable("TDLib is not ready: " + f.message);
	if (o.outcome == "timeout")
		throw InternalApi::Error::timeout("Telegram did not answer in time");
}

/* At most one Telegram call per chat this often for a refresh. */
constexpr std::chrono::seconds kRefreshInterval = 60s;

nlohmann::json refreshAnswer(const DB::GroupMemberInfo &row, bool is_channel,
			     bool is_admin)
{
	return {
		{ "member_count", row.member_count ? nlohmann::json(*row.member_count)
						   : nlohmann::json(nullptr) },
		{ "can_get_members", row.can_get_members.value_or(false) },
		{ "is_channel", is_channel },
		{ "account_is_admin", is_admin },
		{ "fetched_at", row.member_count_at ? nlohmann::json(row.member_count_at)
						    : nlohmann::json(nullptr) },
	};
}

/*
 * POST /v1/chats/{chat_id}/refresh: the group's member count now. One
 * full-info request to Telegram, at most once a minute per chat; within the
 * minute, or when Telegram fails, the stored row answers.
 */
InternalApi::Response refreshChat(TDLib &t, DB &db, const InternalApi::Request &req)
{
	static std::mutex mtx;
	static std::map<int64_t, std::chrono::steady_clock::time_point> last;

	const int64_t chat_id = requireChatId(req);
	auto row = db.getGroupMemberInfo(chat_id);
	if (!row)
		throw InternalApi::Error::notFound("no such group");

	GroupSide g;
	try {
		g = groupSide(t, chat_id);
	} catch (const TdFailure &f) {
		throwIfApiFailure(f);
		/* TDLib does not know the chat now: the stored row. */
		InternalApi::Response r;
		r.body = refreshAnswer(*row, row->type == "channel", false);
		r.body["error"] = f.message;
		return r;
	}

	bool ask = false;
	{
		const auto now = std::chrono::steady_clock::now();
		std::lock_guard<std::mutex> lk(mtx);
		auto it = last.find(chat_id);
		if (it == last.end() || now - it->second >= kRefreshInterval) {
			last[chat_id] = now;
			ask = true;
		}
	}

	nlohmann::json err;
	if (ask) {
		try {
			std::optional<int64_t> count;
			std::optional<bool> can;
			if (g.is_supergroup) {
				auto fi = call<td_api::supergroupFullInfo>(
					t, td_api::make_object<td_api::getSupergroupFullInfo>(
						   g.group_id));
				if (fi->member_count_ > 0)
					count = fi->member_count_;
				can = fi->can_get_members_;
			} else {
				auto fi = call<td_api::basicGroupFullInfo>(
					t, td_api::make_object<td_api::getBasicGroupFullInfo>(
						   g.group_id));
				if (!fi->members_.empty())
					count = (int64_t)fi->members_.size();
				can = !fi->members_.empty();
			}
			db.setGroupMemberInfo(chat_id, count, can);
			row = db.getGroupMemberInfo(chat_id);
		} catch (const TdFailure &f) {
			throwIfApiFailure(f);
			err = f.message;
		}
	}

	InternalApi::Response r;
	r.body = refreshAnswer(*row, g.is_channel, g.is_admin);
	if (!err.is_null())
		r.body["error"] = err;
	return r;
}

/* How long a members answer is reused for the same chat, offset, limit. */
constexpr std::chrono::seconds kMembersCacheTtl = 120s;

nlohmann::json membersUnavailable(const char *reason, std::optional<int64_t> total)
{
	return {
		{ "available", false },
		{ "reason", reason },
		{ "total", total ? nlohmann::json(*total) : nlohmann::json(nullptr) },
		{ "members", nlohmann::json::array() },
	};
}

/* A chat member as the answer lists it, or null for one that is not a
 * current member or not a user (an anonymous admin's chat, a channel). */
nlohmann::json memberJson(const td_api::chatMember &m)
{
	if (!m.member_id_ || m.member_id_->get_id() != td_api::messageSenderUser::ID ||
	    !m.status_)
		return nullptr;
	const char *status = nullptr;
	switch (m.status_->get_id()) {
	case td_api::chatMemberStatusCreator::ID:
		status = "creator";
		break;
	case td_api::chatMemberStatusAdministrator::ID:
		status = "administrator";
		break;
	case td_api::chatMemberStatusMember::ID:
		status = "member";
		break;
	case td_api::chatMemberStatusRestricted::ID:
		if (static_cast<const td_api::chatMemberStatusRestricted &>(
			    *m.status_).is_member_)
			status = "restricted";
		break;
	default:
		break;
	}
	if (!status)
		return nullptr;
	return {
		{ "user_id", static_cast<const td_api::messageSenderUser &>(
				     *m.member_id_).user_id_ },
		{ "status", status },
		{ "joined_at", m.joined_chat_date_ > 0 ? nlohmann::json(m.joined_chat_date_)
						       : nlohmann::json(nullptr) },
	};
}

/*
 * GET /v1/chats/{chat_id}/members: one page of a group's members.
 * Supergroups and channels: getSupergroupMembers, recent filter (channels
 * only where the account is an administrator). Basic groups: the full
 * info's member list, paged here. The users are stored by the daemon's
 * usual user handling: TDLib sends updateUser for every user before an
 * answer mentions it.
 */
InternalApi::Response groupMembers(TDLib &t, DB &db, const InternalApi::Request &req)
{
	static std::mutex mtx;
	struct Cached {
		std::chrono::steady_clock::time_point	at;
		nlohmann::json				body;
	};
	static std::map<std::tuple<int64_t, int64_t, int64_t>, Cached> cache;

	const int64_t chat_id = requireChatId(req);
	const int64_t offset = queryInt(req, "offset", 0, 0, 1000000);
	const int64_t limit = queryInt(req, "limit", 50, 1, 200);
	const auto key = std::make_tuple(chat_id, offset, limit);
	const auto now = std::chrono::steady_clock::now();
	{
		std::lock_guard<std::mutex> lk(mtx);
		auto it = cache.find(key);
		if (it != cache.end() && now - it->second.at < kMembersCacheTtl) {
			InternalApi::Response r;
			r.body = it->second.body;
			return r;
		}
	}

	auto row = db.getGroupMemberInfo(chat_id);
	if (!row)
		throw InternalApi::Error::notFound("no such group");

	nlohmann::json body;
	try {
		GroupSide g = groupSide(t, chat_id);
		if (g.is_channel && !g.is_admin) {
			body = membersUnavailable("not_allowed", row->member_count);
		} else if (!g.is_member) {
			body = membersUnavailable("not_member", row->member_count);
		} else if (g.is_supergroup) {
			auto cm = call<td_api::chatMembers>(
				t, td_api::make_object<td_api::getSupergroupMembers>(
					   g.group_id,
					   td_api::make_object<td_api::supergroupMembersFilterRecent>(),
					   (int32_t)offset, (int32_t)limit));
			nlohmann::json members = nlohmann::json::array();
			for (const auto &m : cm->members_) {
				if (!m)
					continue;
				nlohmann::json e = memberJson(*m);
				if (!e.is_null())
					members.push_back(e);
			}
			body = { { "available", true }, { "reason", "" },
				 { "total", cm->total_count_ }, { "members", members } };
			if (cm->total_count_ > 0)
				db.setGroupMemberInfo(chat_id, (int64_t)cm->total_count_, true);
		} else {
			auto fi = call<td_api::basicGroupFullInfo>(
				t, td_api::make_object<td_api::getBasicGroupFullInfo>(g.group_id));
			nlohmann::json members = nlohmann::json::array();
			for (size_t i = (size_t)offset;
			     i < fi->members_.size() && members.size() < (size_t)limit; i++) {
				if (!fi->members_[i])
					continue;
				nlohmann::json e = memberJson(*fi->members_[i]);
				if (!e.is_null())
					members.push_back(e);
			}
			if (fi->members_.empty()) {
				body = membersUnavailable("not_allowed", row->member_count);
			} else {
				body = { { "available", true }, { "reason", "" },
					 { "total", (int64_t)fi->members_.size() },
					 { "members", members } };
				db.setGroupMemberInfo(chat_id, (int64_t)fi->members_.size(), true);
			}
		}
	} catch (const TdFailure &f) {
		throwIfApiFailure(f);
		ChatOutcome o = classifyChatError(f.code, f.message);
		if (o.outcome == "flood_wait") {
			body = membersUnavailable("flood_wait", row->member_count);
			body["retry_after"] = o.retry_after;
		} else if (f.message.find("CHAT_ADMIN_REQUIRED") != std::string::npos ||
			   f.message.find("rights") != std::string::npos) {
			body = membersUnavailable("not_allowed", row->member_count);
		} else if (o.outcome == "banned") {
			body = membersUnavailable("not_member", row->member_count);
		} else {
			body = membersUnavailable("error", row->member_count);
			body["message"] = f.message;
		}
	}

	{
		std::lock_guard<std::mutex> lk(mtx);
		for (auto it = cache.begin(); it != cache.end();)
			it = now - it->second.at >= kMembersCacheTtl ? cache.erase(it)
								     : std::next(it);
		/* A rate limit or a failure is not kept: the next call asks
		 * again (Go's own cache paces it). */
		if (body["available"].get<bool>() ||
		    body["reason"] == "not_allowed" || body["reason"] == "not_member")
			cache[key] = Cached{ now, body };
	}

	InternalApi::Response r;
	r.body = std::move(body);
	return r;
}

} /* namespace */

void registerChatApi(InternalApi &api, TDLib &tdlib, DB &db)
{
	TDLib *t = &tdlib;
	DB *d = &db;

	api.route("POST", "/v1/chats/{chat_id}/refresh",
		  [t, d](const InternalApi::Request &req) {
		return refreshChat(*t, *d, req);
	});

	api.route("GET", "/v1/chats/{chat_id}/members",
		  [t, d](const InternalApi::Request &req) {
		return groupMembers(*t, *d, req);
	});

	api.route("POST", "/v1/users/resolve", [t](const InternalApi::Request &req) {
		return resolveUser(*t, req);
	});

	api.route("POST", "/v1/chats/resolve", [t](const InternalApi::Request &req) {
		ChatRef ref = requireRef(req);
		ChatInfo ci;
		try {
			ci = preview(*t, ref);
		} catch (const TdFailure &f) {
			return failure(ref, f, std::nullopt);
		}
		if (!ci.joinable())
			return answer(ref, "not_a_group",
				      "that username is a user or a bot, not a "
				      "group or channel", ci);
		return answer(ref, "ok", "", ci);
	});

	api.route("POST", "/v1/chats/join", [t](const InternalApi::Request &req) {
		/*
		 * One join at a time: two admins confirming the same chat
		 * must not both reach Telegram, and joins are what flood
		 * limits punish.
		 */
		static std::mutex join_mtx;
		std::lock_guard<std::mutex> lk(join_mtx);

		ChatRef ref = requireRef(req);
		std::optional<ChatInfo> ci;
		try {
			ci = preview(*t, ref);
		} catch (const TdFailure &f) {
			return failure(ref, f, std::nullopt);
		}
		if (!ci->joinable())
			return answer(ref, "not_a_group",
				      "that username is a user or a bot, not a "
				      "group or channel", *ci);
		if (ci->is_member)
			return answer(ref, "already_member",
				      "the account is already a member", *ci);
		if (ci->banned)
			return answer(ref, "banned",
				      "the account is banned from this chat", *ci);

		/* A ChatJoinResult, one of the four cases below. */
		TDLib::TdObject res;
		try {
			if (ref.kind == ChatRef::Kind::Invite)
				res = callAny(*t, td_api::make_object<
						     td_api::joinChatByInviteLink>(
						     ref.canonical()));
			else
				res = callAny(*t, td_api::make_object<
						     td_api::joinChat>(ci->chat_id));
		} catch (const TdFailure &f) {
			return failure(ref, f, ci);
		}

		switch (res->get_id()) {
		case td_api::chatJoinResultSuccess::ID: {
			auto &s = static_cast<const td_api::chatJoinResultSuccess &>(
				*res);
			ci->chat_id = s.chat_id_;
			ci->is_member = true;
			return answer(ref, "joined", "joined", *ci);
		}
		case td_api::chatJoinResultRequestSent::ID:
			return answer(ref, "request_sent",
				      "join request sent; an administrator of "
				      "the chat must approve it", *ci);
		case td_api::chatJoinResultGuardBotApprovalRequired::ID:
			return answer(ref, "approval_required",
				      "the chat's guard bot must approve the "
				      "join; finish it in a Telegram client", *ci);
		case td_api::chatJoinResultDeclined::ID:
			return answer(ref, "declined",
				      "the chat's guard bot declined the join",
				      *ci);
		default:
			return answer(ref, "error",
				      "unexpected join result from TDLib", *ci);
		}
	});
}

} /* namespace tgloggerd */
