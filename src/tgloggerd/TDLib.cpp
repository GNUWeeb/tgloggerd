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
#include <vector>
#include <cstdint>
#include <iostream>
#include <functional>
#include <ctime>
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

	m.accent_color_id = u.accent_color_id_;
	m.background_custom_emoji_id = u.background_custom_emoji_id_;
	m.profile_accent_color_id = u.profile_accent_color_id_;
	m.profile_background_custom_emoji_id = u.profile_background_custom_emoji_id_;

	m.is_premium = u.is_premium_;
	m.is_support = u.is_support_;
	m.restricts_new_chats = u.restricts_new_chats_;
	m.paid_message_star_count = u.paid_message_star_count_;

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
		case td_api::userTypeBot::ID:
			m.type = models::UserType::Bot;
			break;
		default:
			m.type = models::UserType::Unknown;
			break;
		}
	}

	return m;
}

/*
 * Whether a chat id refers to a private (one-to-one) chat.
 *
 * In TDLib a private chat's id equals the peer user id and is always
 * positive, whereas basic groups, supergroups and channels use negative
 * ids. Secret chats are disabled (use_secret_chats_ = false), so a
 * positive id is unambiguously a private chat. Only private chats are
 * logged to private_messages, whose chat_id foreign key references
 * users.id; routing group/channel messages there (negative chat ids) is
 * what violates that constraint.
 */
bool is_private_chat(int64_t chat_id)
{
	return chat_id > 0;
}

/*
 * Map a td_api::message's content to the coarse MessageContent used by
 * both private and group messages. Media files are not linked here; only
 * the content type (and text, for text messages) is captured.
 */
void extract_message_content(const td_api::message &message,
			     models::MessageContent &out)
{
	if (!message.content_)
		return;

	switch (message.content_->get_id()) {
	case td_api::messageText::ID: {
		auto &c = static_cast<const td_api::messageText &>(
			*message.content_);
		out.content_type = models::MessageContentType::Text;
		if (c.text_)
			out.text = c.text_->text_;
		break;
	}
	case td_api::messagePhoto::ID:
		out.content_type = models::MessageContentType::Photo;
		break;
	case td_api::messageVideo::ID:
		out.content_type = models::MessageContentType::Video;
		break;
	case td_api::messageDocument::ID:
		out.content_type = models::MessageContentType::Document;
		break;
	case td_api::messageAudio::ID:
		out.content_type = models::MessageContentType::Audio;
		break;
	case td_api::messageVoiceNote::ID:
		out.content_type = models::MessageContentType::Voice;
		break;
	case td_api::messageSticker::ID:
		out.content_type = models::MessageContentType::Sticker;
		break;
	case td_api::messageAnimation::ID:
		out.content_type = models::MessageContentType::Animation;
		break;
	default:
		out.content_type = models::MessageContentType::Unknown;
		break;
	}
}

/*
 * Extract forwarded-message origin info from a td_api::message. Returns
 * nullopt for non-forwarded messages. Shared by private and group
 * messages, whose *_fwd_info tables are identical.
 */
std::optional<models::ForwardInfo>
extract_forward_info(const td_api::message &message)
{
	if (!message.forward_info_)
		return std::nullopt;

	models::ForwardInfo fi;
	fi.origin_date = message.forward_info_->date_;

	if (message.forward_info_->origin_) {
		switch (message.forward_info_->origin_->get_id()) {
		case td_api::messageOriginUser::ID: {
			auto &o = static_cast<const td_api::messageOriginUser &>(
				*message.forward_info_->origin_);
			fi.origin_type = models::ForwardOriginType::User;
			fi.origin_sender_user_id = o.sender_user_id_;
			break;
		}
		case td_api::messageOriginHiddenUser::ID: {
			auto &o = static_cast<
				const td_api::messageOriginHiddenUser &>(
				*message.forward_info_->origin_);
			fi.origin_type = models::ForwardOriginType::HiddenUser;
			fi.origin_sender_name = o.sender_name_;
			break;
		}
		case td_api::messageOriginChat::ID: {
			auto &o = static_cast<const td_api::messageOriginChat &>(
				*message.forward_info_->origin_);
			fi.origin_type = models::ForwardOriginType::Chat;
			fi.origin_chat_id = o.sender_chat_id_;
			if (!o.author_signature_.empty())
				fi.origin_sender_name = o.author_signature_;
			break;
		}
		case td_api::messageOriginChannel::ID: {
			auto &o = static_cast<
				const td_api::messageOriginChannel &>(
				*message.forward_info_->origin_);
			fi.origin_type = models::ForwardOriginType::Channel;
			fi.origin_chat_id = o.chat_id_;
			fi.origin_message_id = o.message_id_;
			if (!o.author_signature_.empty())
				fi.origin_sender_name = o.author_signature_;
			break;
		}
		default:
			break;
		}
	}

	return fi;
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
	std::function<void(const models::PrivateMessage &)> private_msg_handler_;
	std::function<void(const models::GroupMessage &)> group_msg_handler_;
	std::function<void(const models::User &)>	user_handler_;
	std::function<void(const ProfilePhoto &)>	photo_handler_;
	std::function<void(const models::Group &)>	group_handler_;
	std::function<void(const GroupPhoto &)>		group_photo_handler_;

	std::unique_ptr<td::ClientManager>		client_manager_;
	td_api::object_ptr<td_api::AuthorizationState>	authorization_state_;

	std::unordered_map<std::uint64_t, std::function<void(Object)>>	handlers_;
	std::unordered_map<int64_t, td_api::object_ptr<td_api::user>>	users_;
	std::unordered_map<int32_t, int64_t>				pending_photo_;

	/* Cached supergroup fields, awaiting the chat to assemble a group. */
	struct SgInfo {
		std::vector<std::string>	active;
		std::vector<std::string>	disabled;
		std::vector<std::string>	collectible;
		bool				is_channel = false;
	};

	/* Assembled group state, merged from the chat/supergroup/full info. */
	struct GroupState {
		models::GroupType		type = models::GroupType::BasicGroup;
		int64_t				chat_id = 0;
		std::string			title;
		std::string			description;
		std::vector<std::string>	active_usernames;
		std::vector<std::string>	disabled_usernames;
		std::vector<std::string>	collectible_usernames;
		bool				chat_seen = false;
	};

	std::unordered_map<int64_t, SgInfo>		supergroups_;
	std::unordered_map<int64_t, GroupState>		group_state_;
	std::unordered_map<int64_t, int64_t>		chat_to_group_;
	std::unordered_map<int32_t, int64_t>		pending_group_photo_;

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
	void handle_message_for_private_chat(td_api::message &message);
	void handle_message_for_group_chat(td_api::message &message);
	void handle_update_message_content(int64_t chat_id, int64_t message_id,
					   const td_api::MessageContent *content);
	void handle_delete_messages(int64_t chat_id,
				    const td_api::array<td_api::int53> &message_ids,
				    bool is_permanent);
	void build_private_message(const td_api::message &message,
				   models::PrivateMessage &out);
	void build_group_message(const td_api::message &message,
				 models::GroupMessage &out);
	void resolve_forward_origin(const models::ForwardInfo &info);
	void ensure_user_saved(int64_t user_id);
	void ensure_chat_saved(int64_t chat_id);
	void maybe_download_profile_photo(const td_api::user &u);
	void handle_file_update(const td_api::file &f);
	void emit_photo(int64_t user_id, const td_api::file &f);
	void handle_new_chat(const td_api::chat &chat);
	void maybe_download_group_photo(int64_t group_id,
					const td_api::chatPhotoInfo *photo);
	void emit_group(int64_t group_id);
	void emit_group_photo(int64_t group_id, const td_api::file &f);
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
			[this](td_api::updateNewChat &u) {
				if (u.chat_)
					handle_new_chat(*u.chat_);
			},
			[this](td_api::updateChatTitle &u) {
				auto it = chat_to_group_.find(u.chat_id_);
				if (it == chat_to_group_.end())
					return;
				group_state_[it->second].title = u.title_;
				emit_group(it->second);
			},
			[this](td_api::updateChatPhoto &u) {
				auto it = chat_to_group_.find(u.chat_id_);
				if (it == chat_to_group_.end())
					return;
				maybe_download_group_photo(u.chat_id_,
							   u.photo_.get());
			},
			[this](td_api::updateSupergroup &u) {
				if (!u.supergroup_)
					return;
				const auto &sg = *u.supergroup_;
				SgInfo info;
				if (sg.usernames_) {
					info.active = sg.usernames_->active_usernames_;
					info.disabled = sg.usernames_->disabled_usernames_;
					info.collectible = sg.usernames_->collectible_usernames_;
				}
				info.is_channel = sg.is_channel_;
				supergroups_[sg.id_] = info;

				auto it = group_state_.find(sg.id_);
				if (it == group_state_.end())
					return;
				it->second.active_usernames = info.active;
				it->second.disabled_usernames = info.disabled;
				it->second.collectible_usernames = info.collectible;
				it->second.type = info.is_channel ?
					models::GroupType::Channel :
					models::GroupType::Supergroup;
				emit_group(sg.id_);
			},
			[this](td_api::updateSupergroupFullInfo &u) {
				if (!u.supergroup_full_info_)
					return;
				auto &st = group_state_[u.supergroup_id_];
				st.description =
					u.supergroup_full_info_->description_;
				if (st.chat_seen)
					emit_group(u.supergroup_id_);
			},
			[this](td_api::updateBasicGroupFullInfo &u) {
				if (!u.basic_group_full_info_)
					return;
				auto &st = group_state_[u.basic_group_id_];
				st.description =
					u.basic_group_full_info_->description_;
				if (st.chat_seen)
					emit_group(u.basic_group_id_);
			},
			[this](td_api::updateNewMessage &u) {
				handle_new_message(*u.message_);
			},
			[this](td_api::updateMessageContent &u) {
				handle_update_message_content(u.chat_id_,
					u.message_id_, u.new_content_.get());
			},
			[this](td_api::updateMessageEdited &u) {
				/*
				 * updateMessageEdited fires with edit_date
				 * but not the content. Request the full
				 * message; when it arrives we rebuild and
				 * upsert it as if it were a new message,
				 * routing to the private or group path by
				 * chat kind.
				 */
				send_query(
					td_api::make_object<td_api::getMessage>(
						u.chat_id_, u.message_id_),
					[this](Object obj) {
						if (obj->get_id() !=
						    td_api::message::ID)
							return;
						auto msg =
							td::move_tl_object_as<
								td_api::message>(obj);
						if (is_private_chat(msg->chat_id_))
							handle_message_for_private_chat(
								*msg);
						else
							handle_message_for_group_chat(
								*msg);
					});
			},
			[this](td_api::updateDeleteMessages &u) {
				handle_delete_messages(
					u.chat_id_,
					u.message_ids_,
					u.is_permanent_);
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
	/*
	 * Route by chat kind: private (positive) chat ids go to
	 * private_messages (chat_id is a users.id); group, supergroup and
	 * channel chats (negative ids) go to group_messages (chat_id is a
	 * groups.id).
	 */
	if (is_private_chat(message.chat_id_))
		handle_message_for_private_chat(message);
	else
		handle_message_for_group_chat(message);

	/* Legacy text-only handler path. */
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

void TDLib::Impl::handle_message_for_private_chat(td_api::message &message)
{
	if (!private_msg_handler_)
		return;

	models::PrivateMessage pm;
	build_private_message(message, pm);
	if (pm.forward_info.has_value())
		resolve_forward_origin(*pm.forward_info);
	private_msg_handler_(pm);
}

void TDLib::Impl::handle_message_for_group_chat(td_api::message &message)
{
	if (!group_msg_handler_)
		return;

	models::GroupMessage gm;
	build_group_message(message, gm);
	if (gm.forward_info.has_value())
		resolve_forward_origin(*gm.forward_info);
	group_msg_handler_(gm);
}

void TDLib::Impl::handle_update_message_content(int64_t chat_id,
						int64_t message_id,
						const td_api::MessageContent *content)
{
	if (!private_msg_handler_)
		return;

	/*
	 * For message edits we need the full message object to get
	 * edit_date and sender. TDLib sends a separate updateMessageEdited
	 * or we can request getMessage. However, updateMessageContent
	 * only has chat_id, message_id, and new_content.
	 *
	 * For now, we send a getMessage query to fetch the full message
	 * so we can determine edit_date. But this adds latency and
	 * complexity. Simpler: just record the content update as a
	 * potential edit. We check private_messages for the existing
	 * row and compare content.
	 *
	 * Actually, the simplest approach for now: we know the message
	 * already exists in private_messages. We can just update the
	 * content fields. But we need edit_date. Let's use a simpler
	 * strategy: request getMessage to get the full object.
	 *
	 * For now, skip updateMessageContent handling; content changes
	 * that TDLib delivers via updateMessageContent without a
	 * corresponding edit_date are initial loads. Real edits come
	 * via updateMessageEdited which we'll handle below, or via
	 * updateNewMessage with an edit_date > 0.
	 *
	 * TODO: implement getMessage-based edit tracking.
	 */
	(void)chat_id;
	(void)message_id;
	(void)content;
}

void TDLib::Impl::handle_delete_messages(int64_t chat_id,
					 const td_api::array<td_api::int53> &message_ids,
					 bool /* is_permanent */)
{
	/* Route deletions to the same table the messages were stored in. */
	bool priv = is_private_chat(chat_id);
	if (priv && !private_msg_handler_)
		return;
	if (!priv && !group_msg_handler_)
		return;

	for (auto msg_id : message_ids) {
		if (priv) {
			models::PrivateMessage pm;
			pm.chat_id = chat_id;
			pm.message_id = msg_id;
			pm.is_deleted = true;
			private_msg_handler_(pm);
		} else {
			models::GroupMessage gm;
			gm.chat_id = chat_id;
			gm.message_id = msg_id;
			gm.is_deleted = true;
			group_msg_handler_(gm);
		}
	}
}

void TDLib::Impl::build_private_message(const td_api::message &message,
					models::PrivateMessage &out)
{
	out.chat_id = message.chat_id_;
	out.message_id = message.id_;
	out.is_outgoing = message.is_outgoing_;
	out.date = message.date_;
	out.edit_date = message.edit_date_;
	out.is_deleted = false;

	/*
	 * Resolve the sender. Messages sent by the logged-in account are
	 * recorded with a NULL sender_id, as the private_messages schema
	 * prescribes. Incoming private-chat messages are always sent by the
	 * peer user; a messageSenderChat is not expected here and is ignored
	 * (its chat id is not a valid users.id).
	 */
	if (!message.is_outgoing_ && message.sender_id_ &&
	    message.sender_id_->get_id() == td_api::messageSenderUser::ID) {
		auto &s = static_cast<const td_api::messageSenderUser &>(
			*message.sender_id_);
		out.sender_id = s.user_id_;
	}

	extract_message_content(message, out.content);
	out.forward_info = extract_forward_info(message);
}

void TDLib::Impl::build_group_message(const td_api::message &message,
				      models::GroupMessage &out)
{
	out.chat_id = message.chat_id_;
	out.message_id = message.id_;
	out.is_outgoing = message.is_outgoing_;
	out.is_channel_post = message.is_channel_post_;
	out.date = message.date_;
	out.edit_date = message.edit_date_;
	out.is_deleted = false;

	if (!message.author_signature_.empty())
		out.author_signature = message.author_signature_;

	/*
	 * A group message's sender may be a user or a chat/channel (channel
	 * posts, anonymous admins). Record whichever applies; the schema
	 * keeps them in separate foreign-key columns. Unlike private
	 * messages, the own account's sender is recorded too (is_outgoing
	 * still marks it), since a group has many participants.
	 */
	if (message.sender_id_) {
		if (message.sender_id_->get_id() ==
		    td_api::messageSenderUser::ID) {
			auto &s = static_cast<const td_api::messageSenderUser &>(
				*message.sender_id_);
			out.sender_user_id = s.user_id_;
		} else if (message.sender_id_->get_id() ==
			   td_api::messageSenderChat::ID) {
			auto &s = static_cast<const td_api::messageSenderChat &>(
				*message.sender_id_);
			out.sender_chat_id = s.chat_id_;
		}
	}

	extract_message_content(message, out.content);
	out.forward_info = extract_forward_info(message);
}

void TDLib::Impl::resolve_forward_origin(const models::ForwardInfo &info)
{
	/*
	 * A forwarded message may reference an entity we have not stored
	 * yet: the original sender (a user) or the original chat/channel.
	 * Memorize it so the *_message_fwd_info references point at real
	 * users/groups rows. Known entities are re-emitted (idempotent, and
	 * it satisfies the origin_sender_user_id foreign key before the
	 * message row is written); unknown ones are fetched from TDLib.
	 */
	if (info.origin_sender_user_id.has_value())
		ensure_user_saved(*info.origin_sender_user_id);
	if (info.origin_chat_id.has_value())
		ensure_chat_saved(*info.origin_chat_id);
}

void TDLib::Impl::ensure_user_saved(int64_t user_id)
{
	if (user_id == 0 || !user_handler_)
		return;

	auto it = users_.find(user_id);
	if (it != users_.end() && it->second) {
		/*
		 * Already known: persist synchronously so the forward-info FK
		 * to users.id resolves before the message row is written.
		 */
		user_handler_(map_user(*it->second));
		return;
	}

	/* Unknown: fetch it, then persist and cache when it arrives. */
	send_query(td_api::make_object<td_api::getUser>(user_id),
		[this](Object obj) {
			if (obj->get_id() != td_api::user::ID)
				return;
			auto u = td::move_tl_object_as<td_api::user>(obj);
			if (user_handler_)
				user_handler_(map_user(*u));
			maybe_download_profile_photo(*u);
			users_[u->id_] = std::move(u);
		});
}

void TDLib::Impl::ensure_chat_saved(int64_t chat_id)
{
	if (chat_id == 0 || !group_handler_)
		return;

	/*
	 * Only group, supergroup and channel chats (negative ids) map to a
	 * groups row; a private chat's peer is handled via the user path.
	 */
	if (is_private_chat(chat_id))
		return;

	auto it = chat_to_group_.find(chat_id);
	if (it != chat_to_group_.end()) {
		/* Already known: re-emit so the group is persisted. */
		emit_group(it->second);
		return;
	}

	/* Unknown: fetch the chat; handle_new_chat persists it. */
	send_query(td_api::make_object<td_api::getChat>(chat_id),
		[this](Object obj) {
			if (obj->get_id() != td_api::chat::ID)
				return;
			auto c = td::move_tl_object_as<td_api::chat>(obj);
			handle_new_chat(*c);
		});
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
	if (!f.local_ || !f.local_->is_downloading_completed_)
		return;

	auto it = pending_photo_.find(f.id_);
	if (it != pending_photo_.end()) {
		int64_t user_id = it->second;
		pending_photo_.erase(it);
		emit_photo(user_id, f);
		return;
	}

	auto git = pending_group_photo_.find(f.id_);
	if (git != pending_group_photo_.end()) {
		int64_t group_id = git->second;
		pending_group_photo_.erase(git);
		emit_group_photo(group_id, f);
		return;
	}
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

void TDLib::Impl::handle_new_chat(const td_api::chat &chat)
{
	if (!chat.type_)
		return;

	int64_t group_id = 0;
	models::GroupType type = models::GroupType::BasicGroup;

	switch (chat.type_->get_id()) {
	case td_api::chatTypeBasicGroup::ID: {
		auto &t = static_cast<const td_api::chatTypeBasicGroup &>(
			*chat.type_);
		group_id = t.basic_group_id_;
		type = models::GroupType::BasicGroup;
		break;
	}
	case td_api::chatTypeSupergroup::ID: {
		auto &t = static_cast<const td_api::chatTypeSupergroup &>(
			*chat.type_);
		group_id = t.supergroup_id_;
		type = t.is_channel_ ? models::GroupType::Channel :
				       models::GroupType::Supergroup;
		break;
	}
	default:
		/* Private and secret chats are not groups. */
		return;
	}

	GroupState &st = group_state_[group_id];
	st.type = type;
	st.chat_id = chat.id_;
	st.title = chat.title_;
	st.chat_seen = true;
	chat_to_group_[chat.id_] = group_id;

	/* Merge usernames already received via updateSupergroup. */
	auto sg = supergroups_.find(group_id);
	if (sg != supergroups_.end()) {
		st.active_usernames = sg->second.active;
		st.disabled_usernames = sg->second.disabled;
		st.collectible_usernames = sg->second.collectible;
		if (sg->second.is_channel)
			st.type = models::GroupType::Channel;
	}

	emit_group(group_id);

	/* Request full info so the description arrives via its update. */
	if (type == models::GroupType::BasicGroup)
		send_query(td_api::make_object<td_api::getBasicGroupFullInfo>(
				   group_id), {});
	else
		send_query(td_api::make_object<td_api::getSupergroupFullInfo>(
				   group_id), {});

	maybe_download_group_photo(chat.id_, chat.photo_.get());
}

void TDLib::Impl::maybe_download_group_photo(int64_t group_id,
					     const td_api::chatPhotoInfo *photo)
{
	if (!group_photo_handler_ || !photo || !photo->big_)
		return;

	const td_api::file &big = *photo->big_;
	if (big.local_ && big.local_->is_downloading_completed_) {
		emit_group_photo(group_id, big);
		return;
	}

	pending_group_photo_[big.id_] = group_id;
	send_query(td_api::make_object<td_api::downloadFile>(
			   big.id_, 1, 0, 0, false), {});
}

void TDLib::Impl::emit_group(int64_t group_id)
{
	auto it = group_state_.find(group_id);
	if (it == group_state_.end() || !it->second.chat_seen ||
	    !group_handler_)
		return;

	const GroupState &st = it->second;
	models::Group g;
	g.id = st.chat_id;
	g.type = st.type;
	g.title = st.title;
	g.description = st.description;
	g.active_usernames = st.active_usernames;
	g.disabled_usernames = st.disabled_usernames;
	g.collectible_usernames = st.collectible_usernames;
	group_handler_(g);
}

void TDLib::Impl::emit_group_photo(int64_t group_id, const td_api::file &f)
{
	if (!group_photo_handler_ || !f.local_ ||
	    !f.local_->is_downloading_completed_)
		return;

	GroupPhoto p;
	p.group_id = group_id;
	p.local_path = f.local_->path_;
	p.tg_file_id = f.remote_ ? f.remote_->id_ : std::string();
	p.file_size = f.size_;
	group_photo_handler_(p);
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

void TDLib::setPrivateMessageHandler(
	std::function<void(const models::PrivateMessage &)> cb)
{
	impl_->private_msg_handler_ = std::move(cb);
}

void TDLib::setGroupMessageHandler(
	std::function<void(const models::GroupMessage &)> cb)
{
	impl_->group_msg_handler_ = std::move(cb);
}

void TDLib::setUserHandler(std::function<void(const models::User &)> cb)
{
	impl_->user_handler_ = std::move(cb);
}

void TDLib::setProfilePhotoHandler(std::function<void(const ProfilePhoto &)> cb)
{
	impl_->photo_handler_ = std::move(cb);
}

void TDLib::setGroupHandler(std::function<void(const models::Group &)> cb)
{
	impl_->group_handler_ = std::move(cb);
}

void TDLib::setGroupPhotoHandler(std::function<void(const GroupPhoto &)> cb)
{
	impl_->group_photo_handler_ = std::move(cb);
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
