// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */
#include "ContactApi.hpp"

#include <td/telegram/td_api.h>
#include <td/telegram/td_api.hpp>

#include <algorithm>
#include <cerrno>
#include <chrono>
#include <cstdlib>
#include <ctime>
#include <string>
#include <utility>
#include <vector>

namespace tgloggerd {

namespace td_api = td::td_api;
using Kind = ContactError::Kind;
using json = nlohmann::json;

namespace {

/* Each TDLib call of the cycle; a removal or re-add waits on Telegram. */
constexpr std::chrono::milliseconds kCallTimeout{15000};

ContactError to_error(const td_api::error &e, const char *what)
{
	std::string msg = std::string(what) + ": " + std::to_string(e.code_) +
			  " " + e.message_;
	if (e.code_ == 429) {
		/* "Too Many Requests: retry after 123" */
		int ra = 0;
		auto p = e.message_.find("retry after ");
		if (p != std::string::npos)
			ra = atoi(e.message_.c_str() + p + 12);
		return ContactError(Kind::FloodWait, msg, std::max(ra, 1));
	}
	if (e.code_ == TDLib::kRequestNotReady)
		return ContactError(Kind::NotReady, msg, 5);
	if (e.code_ == TDLib::kRequestTimeout)
		return ContactError(Kind::Timeout, msg);
	if (e.code_ == 400 && (e.message_.find("USER_NOT_FOUND") !=
				       std::string::npos ||
			       e.message_.find("User not found") !=
				       std::string::npos))
		return ContactError(Kind::NotFound, msg);
	return ContactError(Kind::Failed, msg);
}

/*
 * The precisions and kinds a contact note's DateTime entity can carry,
 * by name, so the journal holds plain JSON.
 */
const char *precision_name(const td_api::DateTimePartPrecision *p)
{
	if (!p)
		return "none";
	switch (p->get_id()) {
	case td_api::dateTimePartPrecisionShort::ID:	return "short";
	case td_api::dateTimePartPrecisionLong::ID:	return "long";
	default:					return "none";
	}
}

td_api::object_ptr<td_api::DateTimePartPrecision>
precision_of(const std::string &s)
{
	if (s == "short")
		return td_api::make_object<td_api::dateTimePartPrecisionShort>();
	if (s == "long")
		return td_api::make_object<td_api::dateTimePartPrecisionLong>();
	return td_api::make_object<td_api::dateTimePartPrecisionNone>();
}

/*
 * userFullInfo.note to JSON. A note may hold only Bold, Italic, Underline,
 * Strikethrough, Spoiler, CustomEmoji and DateTime entities (addContact's
 * contract); anything else could not be put back, so the lookup refuses.
 */
std::string note_to_json(const td_api::formattedText &ft)
{
	json ents = json::array();
	for (const auto &e : ft.entities_) {
		if (!e || !e->type_)
			continue;
		json j = { { "offset", e->offset_ }, { "length", e->length_ } };
		switch (e->type_->get_id()) {
		case td_api::textEntityTypeBold::ID:
			j["type"] = "bold";
			break;
		case td_api::textEntityTypeItalic::ID:
			j["type"] = "italic";
			break;
		case td_api::textEntityTypeUnderline::ID:
			j["type"] = "underline";
			break;
		case td_api::textEntityTypeStrikethrough::ID:
			j["type"] = "strikethrough";
			break;
		case td_api::textEntityTypeSpoiler::ID:
			j["type"] = "spoiler";
			break;
		case td_api::textEntityTypeCustomEmoji::ID: {
			auto &t = static_cast<const td_api::textEntityTypeCustomEmoji &>(
				*e->type_);
			j["type"] = "custom_emoji";
			j["custom_emoji_id"] = t.custom_emoji_id_;
			break;
		}
		case td_api::textEntityTypeDateTime::ID: {
			auto &t = static_cast<const td_api::textEntityTypeDateTime &>(
				*e->type_);
			j["type"] = "date_time";
			j["unix_time"] = t.unix_time_;
			json f = { { "kind", "relative" } };
			if (t.formatting_type_ &&
			    t.formatting_type_->get_id() ==
				    td_api::dateTimeFormattingTypeAbsolute::ID) {
				auto &a = static_cast<const td_api::dateTimeFormattingTypeAbsolute &>(
					*t.formatting_type_);
				f = { { "kind", "absolute" },
				      { "time_precision",
					precision_name(a.time_precision_.get()) },
				      { "date_precision",
					precision_name(a.date_precision_.get()) },
				      { "show_day_of_week", a.show_day_of_week_ } };
			}
			j["formatting"] = std::move(f);
			break;
		}
		default:
			throw ContactError(Kind::Unsupported,
					   "the contact note has formatting"
					   " the re-add could not restore");
		}
		ents.push_back(std::move(j));
	}
	return json{ { "text", ft.text_ }, { "entities", std::move(ents) } }
		.dump();
}

td_api::object_ptr<td_api::formattedText> note_from_json(const std::string &s)
{
	json n = json::parse(s);
	std::vector<td_api::object_ptr<td_api::textEntity>> ents;
	for (const auto &j : n.at("entities")) {
		const std::string type = j.at("type").get<std::string>();
		td_api::object_ptr<td_api::TextEntityType> t;
		if (type == "bold")
			t = td_api::make_object<td_api::textEntityTypeBold>();
		else if (type == "italic")
			t = td_api::make_object<td_api::textEntityTypeItalic>();
		else if (type == "underline")
			t = td_api::make_object<td_api::textEntityTypeUnderline>();
		else if (type == "strikethrough")
			t = td_api::make_object<td_api::textEntityTypeStrikethrough>();
		else if (type == "spoiler")
			t = td_api::make_object<td_api::textEntityTypeSpoiler>();
		else if (type == "custom_emoji")
			t = td_api::make_object<td_api::textEntityTypeCustomEmoji>(
				j.at("custom_emoji_id").get<int64_t>());
		else if (type == "date_time") {
			const json &f = j.at("formatting");
			td_api::object_ptr<td_api::DateTimeFormattingType> ft;
			if (f.at("kind").get<std::string>() == "absolute")
				ft = td_api::make_object<td_api::dateTimeFormattingTypeAbsolute>(
					precision_of(f.at("time_precision")),
					precision_of(f.at("date_precision")),
					f.at("show_day_of_week").get<bool>());
			else
				ft = td_api::make_object<td_api::dateTimeFormattingTypeRelative>();
			t = td_api::make_object<td_api::textEntityTypeDateTime>(
				j.at("unix_time").get<int32_t>(), std::move(ft));
		} else
			continue;
		ents.push_back(td_api::make_object<td_api::textEntity>(
			j.at("offset").get<int32_t>(),
			j.at("length").get<int32_t>(), std::move(t)));
	}
	return td_api::make_object<td_api::formattedText>(
		n.at("text").get<std::string>(), std::move(ents));
}

class TdContactTelegram : public ContactTelegram {
public:
	explicit TdContactTelegram(TDLib &td) : td_(td) {}

	ContactUserState getUser(int64_t user_id) override
	{
		auto o = call(td_api::make_object<td_api::getUser>(user_id),
			      "getUser", td_api::user::ID);
		auto &u = static_cast<td_api::user &>(*o);
		ContactUserState s;
		s.is_contact = u.is_contact_;
		s.first_name = u.first_name_;
		s.last_name = u.last_name_;
		s.phone_number = u.phone_number_;
		return s;
	}

	FullInfo getFullInfo(int64_t user_id) override
	{
		auto o = call(td_api::make_object<td_api::getUserFullInfo>(user_id),
			      "getUserFullInfo", td_api::userFullInfo::ID);
		auto &fi = static_cast<td_api::userFullInfo &>(*o);
		FullInfo r;
		r.need_phone_number_privacy_exception =
			fi.need_phone_number_privacy_exception_;
		if (fi.note_ && !fi.note_->text_.empty())
			r.note_json = note_to_json(*fi.note_);
		return r;
	}

	void removeContact(int64_t user_id) override
	{
		call(td_api::make_object<td_api::removeContacts>(
			     std::vector<int64_t>{ user_id }),
		     "removeContacts", td_api::ok::ID);
	}

	void addContact(const ContactRecord &c) override
	{
		td_api::object_ptr<td_api::formattedText> note;
		if (c.note_json) {
			try {
				note = note_from_json(*c.note_json);
			} catch (const json::exception &e) {
				throw ContactError(Kind::Failed,
						   std::string("journalled note: ") +
						   e.what());
			}
		}
		call(td_api::make_object<td_api::addContact>(
			     c.user_id,
			     td_api::make_object<td_api::importedContact>(
				     c.phone_number, c.first_name, c.last_name,
				     std::move(note)),
			     c.share_phone_number),
		     "addContact", td_api::ok::ID);
	}

private:
	TDLib::TdObject call(TDLib::TdFunction f, const char *what, int32_t want)
	{
		TDLib::TdObject o = td_.request(std::move(f), kCallTimeout);
		if (!o)
			throw ContactError(Kind::Failed,
					   std::string(what) + ": no answer");
		if (o->get_id() == td_api::error::ID)
			throw to_error(static_cast<td_api::error &>(*o), what);
		if (o->get_id() != want)
			throw ContactError(Kind::Failed, std::string(what) +
					   ": unexpected answer");
		return o;
	}

	TDLib	&td_;
};

class DbContactJournal : public ContactJournal {
public:
	explicit DbContactJournal(DB &db) : db_(db) {}

	void insert(const ContactRecord &c) override
	{
		db_.insertContactLookup(to_row(c));
	}
	void markRemoved(int64_t user_id) override
	{
		db_.markContactLookupRemoved(user_id);
	}
	void noteFailure(int64_t user_id, const std::string &err) override
	{
		db_.noteContactLookupFailure(user_id, err);
	}
	void erase(int64_t user_id) override
	{
		db_.deleteContactLookup(user_id);
	}
	std::vector<ContactRecord> pending(void) override
	{
		std::vector<ContactRecord> out;
		for (auto &r : db_.listContactLookups()) {
			ContactRecord c;
			c.user_id = r.user_id;
			c.phone_number = std::move(r.phone_number);
			c.first_name = std::move(r.first_name);
			c.last_name = std::move(r.last_name);
			c.note_json = std::move(r.note_json);
			c.share_phone_number = r.share_phone_number;
			out.push_back(std::move(c));
		}
		return out;
	}

private:
	static DB::ContactLookupRow to_row(const ContactRecord &c)
	{
		DB::ContactLookupRow r;
		r.user_id = c.user_id;
		r.phone_number = c.phone_number;
		r.first_name = c.first_name;
		r.last_name = c.last_name;
		r.note_json = c.note_json;
		r.share_phone_number = c.share_phone_number;
		return r;
	}

	DB	&db_;
};

int64_t parse_user_id(const std::string &s)
{
	bool digits = !s.empty() && s.size() <= 19 &&
		std::all_of(s.begin(), s.end(),
			    [](char c) { return c >= '0' && c <= '9'; });
	if (digits) {
		errno = 0;
		long long v = strtoll(s.c_str(), nullptr, 10);
		if (errno == 0 && v > 0)
			return (int64_t)v;
	}
	throw InternalApi::Error::badRequest("bad user id");
}

InternalApi::Error to_api_error(const ContactError &e)
{
	switch (e.kind()) {
	case Kind::NotFound:
		return InternalApi::Error::notFound(e.what());
	case Kind::NotContact:
		return InternalApi::Error(409, "not_a_contact", e.what());
	case Kind::Unsupported:
		return InternalApi::Error(409, "unsupported", e.what());
	case Kind::RateLimited:
		return InternalApi::Error(429, "rate_limited", e.what(),
					  e.retryAfter());
	case Kind::FloodWait:
		return InternalApi::Error(429, "flood_wait", e.what(),
					  e.retryAfter());
	case Kind::Busy:
	case Kind::NotReady:
		return InternalApi::Error::unavailable(e.what(),
						       std::max(e.retryAfter(), 1));
	case Kind::RestorePending:
		return InternalApi::Error(503, "restore_pending", e.what(),
					  std::max(e.retryAfter(), 1));
	case Kind::Timeout:
		return InternalApi::Error::timeout(e.what());
	case Kind::Failed:
		return InternalApi::Error(502, "telegram_error", e.what());
	case Kind::Internal:
		break;
	}
	return InternalApi::Error(500, "internal", e.what());
}

} /* namespace */

Contacts::Contacts(TDLib &tdlib, DB &db, ContactLookup::StoreOwnName store,
		   ContactLookup::Config cfg, log_hd_t *l)
	: tg_(std::make_unique<TdContactTelegram>(tdlib)),
	  journal_(std::make_unique<DbContactJournal>(db)),
	  lookup_(std::make_unique<ContactLookup>(*tg_, *journal_,
						  std::move(store), cfg, l))
{
}

Contacts::~Contacts(void)
{
	/* Stop the lookup's thread before what it uses goes. */
	lookup_.reset();
}

void registerContactApi(InternalApi &api, Contacts &contacts)
{
	ContactLookup *cl = &contacts.lookup();

	api.route("POST", "/v1/users/{id}/lookup-name",
		  [cl](const InternalApi::Request &req) {
		int64_t id = parse_user_id(req.param("id"));
		ContactLookup::Result r;
		try {
			r = cl->lookup(id);
		} catch (const ContactError &e) {
			throw to_api_error(e);
		}

		InternalApi::Response resp;
		resp.body = {
			{ "user_id", r.user_id },
			{ "first_name", r.own_name.first },
			{ "last_name", r.own_name.second },
			{ "contact_first_name", r.contact_name.first },
			{ "contact_last_name", r.contact_name.second },
			{ "checked_at", (int64_t)time(nullptr) },
		};
		return resp;
	});
}

} /* namespace tgloggerd */
