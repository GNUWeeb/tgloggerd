// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ChatApi.hpp"

#include "ChatRef.hpp"
#include "TDLib.hpp"

#include <td/telegram/td_api.h>

#include <chrono>
#include <mutex>
#include <optional>
#include <string>

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

} /* namespace */

void registerChatApi(InternalApi &api, TDLib &tdlib)
{
	TDLib *t = &tdlib;

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
