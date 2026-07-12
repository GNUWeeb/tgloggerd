// SPDX-License-Identifier: GPL-2.0-only
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
#include <tgloggerd/TDLib.hpp>

#include <td/telegram/Client.h>
#include <td/telegram/td_api.h>
#include <td/telegram/td_api.hpp>

#include <string>
#include <memory>
#include <cstdint>
#include <iostream>
#include <functional>
#include <unordered_map>

namespace tgloggerd {

namespace td_api = td::td_api;

namespace {

using Object = td_api::object_ptr<td_api::Object>;

/*
 * Helper to combine a set of lambdas into a single overload set that can
 * be handed to td_api::downcast_call.
 */
template <class... Fs>
struct overload;

template <class F>
struct overload<F> : public F {
	explicit overload(F f) : F(f) {}
};

template <class F, class... Fs>
struct overload<F, Fs...> : public overload<F>, overload<Fs...> {
	overload(F f, Fs... fs) : overload<F>(f), overload<Fs...>(fs...) {}
	using overload<F>::operator();
	using overload<Fs...>::operator();
};

template <class... F>
static auto overloaded(F... f)
{
	return overload<F...>(f...);
}

models::User map_user(const td_api::user &u)
{
	models::User m;

	m.id = u.id_;
	m.first_name = u.first_name_;
	m.last_name = u.last_name_;
	m.phone_number = u.phone_number_;
	m.language_code = u.language_code_;

	m.accent_color_id = u.accent_color_id_;
	m.background_custom_emoji_id = u.background_custom_emoji_id_;
	m.profile_accent_color_id = u.profile_accent_color_id_;
	m.profile_background_custom_emoji_id = u.profile_background_custom_emoji_id_;

	m.is_contact = u.is_contact_;
	m.is_mutual_contact = u.is_mutual_contact_;
	m.is_close_friend = u.is_close_friend_;
	m.is_premium = u.is_premium_;
	m.is_support = u.is_support_;
	m.restricts_new_chats = u.restricts_new_chats_;
	m.paid_message_star_count = u.paid_message_star_count_;
	m.have_access = u.have_access_;
	m.added_to_attachment_menu = u.added_to_attachment_menu_;

	if (u.verification_status_) {
		m.is_verified = u.verification_status_->is_verified_;
		m.is_scam = u.verification_status_->is_scam_;
		m.is_fake = u.verification_status_->is_fake_;
	}

	if (u.restriction_info_) {
		m.restriction_reason = u.restriction_info_->restriction_reason_;
		m.has_sensitive_content = u.restriction_info_->has_sensitive_content_;
	}

	if (u.emoji_status_ && u.emoji_status_->type_ &&
	    u.emoji_status_->type_->get_id() ==
		    td_api::emojiStatusTypeCustomEmoji::ID) {
		auto &t = static_cast<const td_api::emojiStatusTypeCustomEmoji &>(
			*u.emoji_status_->type_);
		m.emoji_status_custom_emoji_id = t.custom_emoji_id_;
		m.emoji_status_expiration_date = u.emoji_status_->expiration_date_;
	}

	if (u.usernames_) {
		m.active_usernames = u.usernames_->active_usernames_;
		m.disabled_usernames = u.usernames_->disabled_usernames_;
		m.editable_username = u.usernames_->editable_username_;
		m.collectible_usernames = u.usernames_->collectible_usernames_;
	}

	m.type = models::UserType::Unknown;
	if (u.type_) {
		switch (u.type_->get_id()) {
		case td_api::userTypeRegular::ID:
			m.type = models::UserType::Regular;
			break;
		case td_api::userTypeDeleted::ID:
			m.type = models::UserType::Deleted;
			break;
		case td_api::userTypeBot::ID: {
			m.type = models::UserType::Bot;
			auto &t = static_cast<const td_api::userTypeBot &>(*u.type_);
			models::BotInfo bi;
			bi.can_be_edited = t.can_be_edited_;
			bi.can_join_groups = t.can_join_groups_;
			bi.can_read_all_group_messages = t.can_read_all_group_messages_;
			bi.has_main_web_app = t.has_main_web_app_;
			bi.has_topics = t.has_topics_;
			bi.allows_users_to_create_topics = t.allows_users_to_create_topics_;
			bi.can_manage_bots = t.can_manage_bots_;
			bi.is_inline = t.is_inline_;
			bi.inline_query_placeholder = t.inline_query_placeholder_;
			bi.supports_guest_queries = t.supports_guest_queries_;
			bi.is_guard = t.is_guard_;
			bi.need_location = t.need_location_;
			bi.can_connect_to_business = t.can_connect_to_business_;
			bi.can_be_added_to_attachment_menu = t.can_be_added_to_attachment_menu_;
			bi.active_user_count = t.active_user_count_;
			m.bot = bi;
			break;
		}
		default:
			m.type = models::UserType::Unknown;
			break;
		}
	}

	return m;
}

} /* namespace */


struct TDLib::Impl {
	uint32_t	api_id_;
	std::string	api_hash_;
	std::string	data_dir_;

	bool		stopped_ = false;
	bool		is_authorized_ = false;
	int64_t		client_id_ = 0;
	int64_t		user_id_ = 0;
	std::uint64_t	current_query_id_ = 0;

	std::function<void(const TextMessage &)>	msg_handler_;
	std::function<void(const models::User &)>	user_handler_;
	std::function<void(const ProfilePhoto &)>	photo_handler_;

	std::unique_ptr<td::ClientManager>		client_manager_;
	td_api::object_ptr<td_api::AuthorizationState>	authorization_state_;

	std::unordered_map<std::uint64_t, std::function<void(Object)>>	handlers_;
	std::unordered_map<int64_t, td_api::object_ptr<td_api::user>>	users_;
	std::unordered_map<int32_t, int64_t>				pending_photo_;

	Impl(uint32_t api_id, const char *api_hash, const char *data_dir);

	std::uint64_t next_query_id(void) { return ++current_query_id_; }

	void send_query(td_api::object_ptr<td_api::Function> f,
			std::function<void(Object)> handler);
	void process_response(td::ClientManager::Response response);
	void process_update(td_api::object_ptr<td_api::Object> update);
	void on_authorization_state_update(void);
	void check_authentication_error(Object object);
	std::function<void(Object)> create_authentication_query_handler(void);
	void handle_new_message(td_api::message &message);
	void maybe_download_profile_photo(const td_api::user &u);
	void handle_file_update(const td_api::file &f);
	void emit_photo(int64_t user_id, const td_api::file &f);
};


TDLib::Impl::Impl(uint32_t api_id, const char *api_hash, const char *data_dir)
	: api_id_(api_id)
	, api_hash_(api_hash)
	, data_dir_(data_dir)
{
	td::ClientManager::execute(
		td_api::make_object<td_api::setLogVerbosityLevel>(1));

	client_manager_ = std::make_unique<td::ClientManager>();
	client_id_ = client_manager_->create_client_id();

	/* Kick off the client so that the first authorization state arrives. */
	send_query(td_api::make_object<td_api::getOption>("version"), {});
}


void TDLib::Impl::send_query(td_api::object_ptr<td_api::Function> f,
			     std::function<void(Object)> handler)
{
	auto query_id = next_query_id();

	if (handler)
		handlers_.emplace(query_id, std::move(handler));

	client_manager_->send(client_id_, query_id, std::move(f));
}


void TDLib::Impl::process_response(td::ClientManager::Response response)
{
	if (!response.object)
		return;

	if (response.request_id == 0) {
		process_update(std::move(response.object));
		return;
	}

	auto it = handlers_.find(response.request_id);
	if (it != handlers_.end()) {
		auto handler = std::move(it->second);
		handlers_.erase(it);
		handler(std::move(response.object));
	}
}


void TDLib::Impl::process_update(td_api::object_ptr<td_api::Object> update)
{
	td_api::downcast_call(
		*update,
		overloaded(
			[this](td_api::updateAuthorizationState &u) {
				authorization_state_ =
					std::move(u.authorization_state_);
				on_authorization_state_update();
			},
			[this](td_api::updateUser &u) {
				if (user_handler_ && u.user_)
					user_handler_(map_user(*u.user_));
				if (u.user_)
					maybe_download_profile_photo(*u.user_);
				users_[u.user_->id_] = std::move(u.user_);
			},
			[this](td_api::updateFile &u) {
				if (u.file_)
					handle_file_update(*u.file_);
			},
			[this](td_api::updateNewMessage &u) {
				handle_new_message(*u.message_);
			},
			[](auto &) {}
		)
	);
}


void TDLib::Impl::on_authorization_state_update(void)
{
	td_api::downcast_call(
		*authorization_state_,
		overloaded(
			[this](td_api::authorizationStateWaitTdlibParameters &) {
				auto p = td_api::make_object<
					td_api::setTdlibParameters>();
				p->database_directory_ = data_dir_;
				p->use_message_database_ = true;
				p->use_secret_chats_ = false;
				p->api_id_ = static_cast<int32_t>(api_id_);
				p->api_hash_ = api_hash_;
				p->system_language_code_ = "en";
				p->device_model_ = "Desktop";
				p->application_version_ = "1.0";
				send_query(std::move(p),
					   create_authentication_query_handler());
			},
			[this](td_api::authorizationStateWaitPhoneNumber &) {
				std::string pn;
				std::cout << "Enter phone number: " << std::flush;
				std::getline(std::cin, pn);
				send_query(td_api::make_object<
					   td_api::setAuthenticationPhoneNumber>(
						   pn, nullptr),
					   create_authentication_query_handler());
			},
			[this](td_api::authorizationStateWaitCode &) {
				std::string code;
				std::cout << "Enter authentication code: "
					  << std::flush;
				std::getline(std::cin, code);
				send_query(td_api::make_object<
					   td_api::checkAuthenticationCode>(code),
					   create_authentication_query_handler());
			},
			[this](td_api::authorizationStateWaitRegistration &) {
				std::string first, last;
				std::cout << "Enter first name: " << std::flush;
				std::getline(std::cin, first);
				std::cout << "Enter last name: " << std::flush;
				std::getline(std::cin, last);
				send_query(td_api::make_object<
					   td_api::registerUser>(first, last,
								 false),
					   create_authentication_query_handler());
			},
			[this](td_api::authorizationStateWaitPassword &) {
				std::string pass;
				std::cout << "Enter password: " << std::flush;
				std::getline(std::cin, pass);
				send_query(td_api::make_object<
					   td_api::checkAuthenticationPassword>(
						   pass),
					   create_authentication_query_handler());
			},
			[this](td_api::authorizationStateReady &) {
				is_authorized_ = true;
				send_query(td_api::make_object<td_api::getMe>(),
					[this](Object obj) {
						if (obj->get_id() !=
						    td_api::user::ID)
							return;
						auto u = td::move_tl_object_as<
							td_api::user>(obj);
						user_id_ = u->id_;
					});
			},
			[this](td_api::authorizationStateLoggingOut &) {
				is_authorized_ = false;
			},
			[](td_api::authorizationStateClosing &) {},
			[this](td_api::authorizationStateClosed &) {
				is_authorized_ = false;
				stopped_ = true;
			},
			[](auto &) {}
		)
	);
}


void TDLib::Impl::check_authentication_error(Object object)
{
	if (object->get_id() == td_api::error::ID) {
		auto error = td::move_tl_object_as<td_api::error>(object);
		std::cerr << "Authentication error: " << error->message_
			  << std::endl;
	}
}


std::function<void(Object)> TDLib::Impl::create_authentication_query_handler(void)
{
	return [this](Object object) {
		check_authentication_error(std::move(object));
	};
}


void TDLib::Impl::handle_new_message(td_api::message &message)
{
	if (!msg_handler_)
		return;

	/* Only text messages are handled for now. */
	if (!message.content_ ||
	    message.content_->get_id() != td_api::messageText::ID)
		return;

	auto &content = static_cast<td_api::messageText &>(*message.content_);
	if (!content.text_)
		return;

	TextMessage msg;
	msg.sender_id = 0;
	msg.message_id = message.id_;
	msg.text = content.text_->text_;

	if (message.sender_id_) {
		if (message.sender_id_->get_id() ==
		    td_api::messageSenderUser::ID) {
			auto &s = static_cast<td_api::messageSenderUser &>(
				*message.sender_id_);
			msg.sender_id = s.user_id_;
		} else if (message.sender_id_->get_id() ==
			   td_api::messageSenderChat::ID) {
			auto &s = static_cast<td_api::messageSenderChat &>(
				*message.sender_id_);
			msg.sender_id = s.chat_id_;
		}
	}

	/* Resolve the sender name and username from the cached users. */
	auto it = users_.find(msg.sender_id);
	if (it != users_.end() && it->second) {
		const auto &user = *it->second;
		msg.sender_name = user.first_name_;
		if (!user.last_name_.empty()) {
			if (!msg.sender_name.empty())
				msg.sender_name += " ";
			msg.sender_name += user.last_name_;
		}
		if (user.usernames_ &&
		    !user.usernames_->active_usernames_.empty())
			msg.sender_username =
				user.usernames_->active_usernames_[0];
	}

	msg_handler_(msg);
}

void TDLib::Impl::maybe_download_profile_photo(const td_api::user &u)
{
	if (!photo_handler_ || !u.profile_photo_ || !u.profile_photo_->big_)
		return;

	const td_api::file &big = *u.profile_photo_->big_;

	/* Already downloaded: emit immediately. */
	if (big.local_ && big.local_->is_downloading_completed_) {
		emit_photo(u.id_, big);
		return;
	}

	/* Otherwise request the download and remember which user it is for. */
	pending_photo_[big.id_] = u.id_;
	send_query(td_api::make_object<td_api::downloadFile>(
			   big.id_, 1, 0, 0, false), {});
}

void TDLib::Impl::handle_file_update(const td_api::file &f)
{
	auto it = pending_photo_.find(f.id_);
	if (it == pending_photo_.end())
		return;
	if (!f.local_ || !f.local_->is_downloading_completed_)
		return;

	int64_t user_id = it->second;
	pending_photo_.erase(it);
	emit_photo(user_id, f);
}

void TDLib::Impl::emit_photo(int64_t user_id, const td_api::file &f)
{
	if (!photo_handler_ || !f.local_ ||
	    !f.local_->is_downloading_completed_)
		return;

	ProfilePhoto p;
	p.user_id = user_id;
	p.local_path = f.local_->path_;
	p.tg_file_id = f.remote_ ? f.remote_->id_ : std::string();
	p.file_size = f.size_;
	photo_handler_(p);
}


TDLib::TDLib(uint32_t api_id, const char *api_hash, const char *data_dir)
	: impl_(std::make_unique<Impl>(api_id, api_hash, data_dir))
{
}

TDLib::~TDLib(void) = default;

void TDLib::setMessageHandler(std::function<void(const TextMessage &)> cb)
{
	impl_->msg_handler_ = std::move(cb);
}

void TDLib::setUserHandler(std::function<void(const models::User &)> cb)
{
	impl_->user_handler_ = std::move(cb);
}

void TDLib::setProfilePhotoHandler(std::function<void(const ProfilePhoto &)> cb)
{
	impl_->photo_handler_ = std::move(cb);
}

void TDLib::loop(int timeout)
{
	impl_->process_response(impl_->client_manager_->receive(timeout));
}

bool TDLib::isStopped(void) const
{
	return impl_->stopped_;
}

void TDLib::close(void)
{
	impl_->send_query(td_api::make_object<td_api::close>(), {});
}

} /* namespace tgloggerd */
