// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
/*
 * Mapping of TDLib message content onto the models stored in the database,
 * kept apart from TDLib.cpp so that it can be tested without a TDLib client.
 */
#include <tgloggerd/MessageExtract.hpp>

#include <string>
#include <optional>
#include <cstdint>

namespace tgloggerd {

namespace {

/* Append @s to @out as a JSON string literal (quotes included), escaping
 * per RFC 8259. Control characters below 0x20 become \u00XX. */
void json_append_string(std::string &out, const std::string &s)
{
	static const char hex[] = "0123456789abcdef";
	out += '"';
	for (unsigned char c : s) {
		switch (c) {
		case '"':  out += "\\\""; break;
		case '\\': out += "\\\\"; break;
		case '\b': out += "\\b"; break;
		case '\f': out += "\\f"; break;
		case '\n': out += "\\n"; break;
		case '\r': out += "\\r"; break;
		case '\t': out += "\\t"; break;
		default:
			if (c < 0x20) {
				out += "\\u00";
				out += hex[(c >> 4) & 0xf];
				out += hex[c & 0xf];
			} else {
				out += (char)c;
			}
		}
	}
	out += '"';
}

/* The stable string stored for a td_api::TextEntityType, matching the
 * subtype without the "textEntityType" prefix, lower-snake-cased. */
const char *entity_type_string(const td_api::TextEntityType &t)
{
	switch (t.get_id()) {
	case td_api::textEntityTypeBold::ID:		return "bold";
	case td_api::textEntityTypeItalic::ID:		return "italic";
	case td_api::textEntityTypeUnderline::ID:	return "underline";
	case td_api::textEntityTypeStrikethrough::ID:	return "strikethrough";
	case td_api::textEntityTypeSpoiler::ID:		return "spoiler";
	case td_api::textEntityTypeCode::ID:		return "code";
	case td_api::textEntityTypePre::ID:		return "pre";
	case td_api::textEntityTypePreCode::ID:		return "pre_code";
	case td_api::textEntityTypeBlockQuote::ID:	return "block_quote";
	case td_api::textEntityTypeExpandableBlockQuote::ID:
							return "expandable_block_quote";
	case td_api::textEntityTypeTextUrl::ID:		return "text_url";
	case td_api::textEntityTypeUrl::ID:		return "url";
	case td_api::textEntityTypeMention::ID:		return "mention";
	case td_api::textEntityTypeMentionName::ID:	return "mention_name";
	case td_api::textEntityTypeHashtag::ID:		return "hashtag";
	case td_api::textEntityTypeCashtag::ID:		return "cashtag";
	case td_api::textEntityTypeBotCommand::ID:	return "bot_command";
	case td_api::textEntityTypeEmailAddress::ID:	return "email";
	case td_api::textEntityTypePhoneNumber::ID:	return "phone_number";
	case td_api::textEntityTypeBankCardNumber::ID:	return "bank_card_number";
	case td_api::textEntityTypeCustomEmoji::ID:	return "custom_emoji";
	case td_api::textEntityTypeMediaTimestamp::ID:	return "media_timestamp";
	default:					return "unknown";
	}
}

/*
 * Serialize a formattedText's entities to a compact JSON array. Each entry
 * is {"offset","length","type", ...type-specific}; offsets/lengths are the
 * UTF-16 code-unit spans TDLib reports. Returns nullopt when there are no
 * entities (so a plain-text message stores SQL NULL).
 */
std::optional<std::string> format_entities_json(const td_api::formattedText &ft)
{
	if (ft.entities_.empty())
		return std::nullopt;

	std::string out = "[";
	bool first = true;
	for (const auto &e : ft.entities_) {
		if (!e || !e->type_)
			continue;
		if (!first)
			out += ',';
		first = false;

		out += "{\"offset\":" + std::to_string(e->offset_);
		out += ",\"length\":" + std::to_string(e->length_);
		out += ",\"type\":";
		json_append_string(out, entity_type_string(*e->type_));

		switch (e->type_->get_id()) {
		case td_api::textEntityTypeTextUrl::ID: {
			auto &t = static_cast<const td_api::textEntityTypeTextUrl &>(
				*e->type_);
			out += ",\"url\":";
			json_append_string(out, t.url_);
			break;
		}
		case td_api::textEntityTypePreCode::ID: {
			auto &t = static_cast<const td_api::textEntityTypePreCode &>(
				*e->type_);
			out += ",\"language\":";
			json_append_string(out, t.language_);
			break;
		}
		case td_api::textEntityTypeMentionName::ID: {
			auto &t = static_cast<const td_api::textEntityTypeMentionName &>(
				*e->type_);
			out += ",\"user_id\":" + std::to_string(t.user_id_);
			break;
		}
		case td_api::textEntityTypeCustomEmoji::ID: {
			auto &t = static_cast<const td_api::textEntityTypeCustomEmoji &>(
				*e->type_);
			out += ",\"custom_emoji_id\":" +
				std::to_string(t.custom_emoji_id_);
			break;
		}
		default:
			break;
		}
		out += '}';
	}
	out += ']';

	/* All entries were malformed and skipped. */
	if (first)
		return std::nullopt;
	return out;
}

/* Store a formattedText (message text or media caption) into @out: the raw
 * text (nullopt if empty) and its formatting entities. */
void set_formatted_text(const td_api::formattedText *ft,
			models::MessageContent &out)
{
	if (!ft)
		return;
	if (!ft->text_.empty())
		out.text = ft->text_;
	out.entities = format_entities_json(*ft);
}

/*
 * Describe a system/service message (a member joined, the title changed,
 * ...). On a recognized service content, sets @service_type to a stable
 * action id and returns a human-readable description (from the account's
 * point of view; specific names are resolved later by the web UI). Returns
 * nullopt when @content is not a service message, so the caller records it
 * as ordinary/unknown content instead.
 */
std::optional<std::string>
describe_service_message(const td_api::message &message,
			 std::string &service_type)
{
	const auto &content = *message.content_;
	switch (content.get_id()) {
	case td_api::messageChatAddMembers::ID: {
		auto &c = static_cast<const td_api::messageChatAddMembers &>(content);
		service_type = "chat_add_members";
		/* Exactly one added member, equal to the sender: a self-join. */
		if (c.member_user_ids_.size() == 1 &&
		    c.member_user_ids_[0] == sender_user_id(message))
			return "joined the group";
		if (c.member_user_ids_.size() == 1)
			return "added a member to the group";
		return "added " + std::to_string(c.member_user_ids_.size()) +
			" members to the group";
	}
	case td_api::messageChatJoinByLink::ID:
		service_type = "chat_join_by_link";
		return "joined the group via invite link";
	case td_api::messageChatJoinByRequest::ID:
		service_type = "chat_join_by_request";
		return "was accepted into the group";
	case td_api::messageChatDeleteMember::ID: {
		auto &c = static_cast<const td_api::messageChatDeleteMember &>(content);
		service_type = "chat_delete_member";
		if (c.user_id_ == sender_user_id(message))
			return "left the group";
		return "removed a member from the group";
	}
	case td_api::messageChatChangeTitle::ID: {
		auto &c = static_cast<const td_api::messageChatChangeTitle &>(content);
		service_type = "chat_change_title";
		return "changed the group name to \"" + c.title_ + "\"";
	}
	case td_api::messageChatChangePhoto::ID:
		service_type = "chat_change_photo";
		return "changed the group photo";
	case td_api::messageChatDeletePhoto::ID:
		service_type = "chat_delete_photo";
		return "removed the group photo";
	case td_api::messagePinMessage::ID:
		service_type = "pin_message";
		return "pinned a message";
	case td_api::messageBasicGroupChatCreate::ID:
	case td_api::messageSupergroupChatCreate::ID:
		service_type = "chat_create";
		return "created the group";
	case td_api::messageChatUpgradeTo::ID:
		service_type = "chat_upgrade_to";
		return "the group was upgraded to a supergroup";
	case td_api::messageChatUpgradeFrom::ID:
		service_type = "chat_upgrade_from";
		return "the supergroup was created from a group";
	case td_api::messageChatSetMessageAutoDeleteTime::ID: {
		auto &c = static_cast<
			const td_api::messageChatSetMessageAutoDeleteTime &>(content);
		service_type = "chat_set_message_auto_delete_time";
		if (c.message_auto_delete_time_ > 0)
			return "set messages to auto-delete after " +
				std::to_string(c.message_auto_delete_time_) +
				" seconds";
		return "disabled auto-delete for messages";
	}
	case td_api::messageChatSetTheme::ID:
		service_type = "chat_set_theme";
		return "changed the chat theme";
	case td_api::messageChatSetBackground::ID:
		service_type = "chat_set_background";
		return "changed the chat background";
	case td_api::messageForumTopicCreated::ID: {
		auto &c = static_cast<const td_api::messageForumTopicCreated &>(content);
		service_type = "forum_topic_created";
		return "created topic \"" + c.name_ + "\"";
	}
	case td_api::messageForumTopicEdited::ID:
		service_type = "forum_topic_edited";
		return "edited a topic";
	case td_api::messageForumTopicIsClosedToggled::ID:
		service_type = "forum_topic_is_closed_toggled";
		return "changed a topic's closed state";
	case td_api::messageForumTopicIsHiddenToggled::ID:
		service_type = "forum_topic_is_hidden_toggled";
		return "changed a topic's hidden state";
	case td_api::messageVideoChatStarted::ID:
		service_type = "video_chat_started";
		return "started a video chat";
	case td_api::messageVideoChatEnded::ID:
		service_type = "video_chat_ended";
		return "ended the video chat";
	case td_api::messageVideoChatScheduled::ID:
		service_type = "video_chat_scheduled";
		return "scheduled a video chat";
	case td_api::messageInviteVideoChatParticipants::ID:
		service_type = "invite_video_chat_participants";
		return "invited participants to the video chat";
	case td_api::messageChatBoost::ID:
		service_type = "chat_boost";
		return "boosted the group";
	case td_api::messageScreenshotTaken::ID:
		service_type = "screenshot_taken";
		return "took a screenshot";
	case td_api::messageContactRegistered::ID:
		service_type = "contact_registered";
		return "joined Telegram";
	case td_api::messageGiftedPremium::ID:
		service_type = "gifted_premium";
		return "gifted a Telegram Premium subscription";
	case td_api::messagePaymentSuccessful::ID:
	case td_api::messagePaymentSuccessfulBot::ID:
		service_type = "payment_successful";
		return "made a payment";
	case td_api::messageCustomServiceAction::ID: {
		auto &c = static_cast<const td_api::messageCustomServiceAction &>(content);
		service_type = "custom_service_action";
		return c.text_;
	}
	default:
		return std::nullopt;
	}
}

} /* namespace */

/* The user id of a message's sender, or 0 if the sender is a chat/channel
 * or absent. Used to tell a self-action (joined/left) from one done to
 * someone else (added/removed). */
int64_t sender_user_id(const td_api::message &message)
{
	if (message.sender_id_ &&
	    message.sender_id_->get_id() == td_api::messageSenderUser::ID)
		return static_cast<const td_api::messageSenderUser &>(
			*message.sender_id_).user_id_;
	return 0;
}

/*
 * Overload taking the message, for the callers that have one. The work is done
 * on the content alone so that updateMessageContent -- which is handed content
 * with no message around it -- can use the same extraction.
 */
void extract_message_content(const td_api::message &message,
			     models::MessageContent &out)
{
	if (message.content_)
		extract_message_content(*message.content_, out, &message);
}

void extract_message_content(const td_api::MessageContent &content,
			     models::MessageContent &out,
			     const td_api::message *msg)
{
	switch (content.get_id()) {
	case td_api::messageText::ID: {
		auto &c = static_cast<const td_api::messageText &>(
			content);
		out.content_type = models::MessageContentType::Text;
		set_formatted_text(c.text_.get(), out);
		break;
	}
	case td_api::messagePhoto::ID: {
		auto &c = static_cast<const td_api::messagePhoto &>(content);
		out.content_type = models::MessageContentType::Photo;
		set_formatted_text(c.caption_.get(), out);
		break;
	}
	case td_api::messageVideo::ID: {
		auto &c = static_cast<const td_api::messageVideo &>(content);
		out.content_type = models::MessageContentType::Video;
		set_formatted_text(c.caption_.get(), out);
		break;
	}
	case td_api::messageDocument::ID: {
		auto &c = static_cast<const td_api::messageDocument &>(content);
		out.content_type = models::MessageContentType::Document;
		set_formatted_text(c.caption_.get(), out);
		break;
	}
	case td_api::messageAudio::ID: {
		auto &c = static_cast<const td_api::messageAudio &>(content);
		out.content_type = models::MessageContentType::Audio;
		set_formatted_text(c.caption_.get(), out);
		break;
	}
	case td_api::messageVoiceNote::ID: {
		auto &c = static_cast<const td_api::messageVoiceNote &>(content);
		out.content_type = models::MessageContentType::Voice;
		set_formatted_text(c.caption_.get(), out);
		break;
	}
	case td_api::messageAnimation::ID: {
		auto &c = static_cast<const td_api::messageAnimation &>(content);
		out.content_type = models::MessageContentType::Animation;
		set_formatted_text(c.caption_.get(), out);
		break;
	}
	case td_api::messageSticker::ID:
		out.content_type = models::MessageContentType::Sticker;
		break;
	case td_api::messageExpiredPhoto::ID:
	case td_api::messageExpiredVideo::ID:
	case td_api::messageExpiredVideoNote::ID:
	case td_api::messageExpiredVoiceNote::ID:
		out.content_type = models::MessageContentType::Unknown;
		out.is_expired = true;
		break;
	default: {
		/* A system/service message (member joined, title changed, ...)
		 * is a real, replyable message; record it as such. Anything
		 * else is genuinely unknown content. */
		/*
		 * Describing a service message needs the message, not just its
		 * content -- it reports who acted. updateMessageContent has no
		 * message to give, but service messages are not edited, so the
		 * only caller without one is also the one that never sees them.
		 */
		std::string service_type;
		std::optional<std::string> text;
		if (msg)
			text = describe_service_message(*msg, service_type);
		if (text.has_value()) {
			out.content_type = models::MessageContentType::Service;
			out.service_type = std::move(service_type);
			if (!text->empty())
				out.text = std::move(*text);
		} else {
			out.content_type = models::MessageContentType::Unknown;
		}
		break;
	}
	}
}

/* The message's text/caption formattedText (carrying its entities), or nullptr
 * for content that has none. Mirrors extract_message_content's text sources. */
const td_api::formattedText *
message_formatted_text(const td_api::message &message)
{
	if (!message.content_)
		return nullptr;
	switch (message.content_->get_id()) {
	case td_api::messageText::ID:
		return static_cast<const td_api::messageText &>(
			*message.content_).text_.get();
	case td_api::messagePhoto::ID:
		return static_cast<const td_api::messagePhoto &>(
			*message.content_).caption_.get();
	case td_api::messageVideo::ID:
		return static_cast<const td_api::messageVideo &>(
			*message.content_).caption_.get();
	case td_api::messageDocument::ID:
		return static_cast<const td_api::messageDocument &>(
			*message.content_).caption_.get();
	case td_api::messageAudio::ID:
		return static_cast<const td_api::messageAudio &>(
			*message.content_).caption_.get();
	case td_api::messageVoiceNote::ID:
		return static_cast<const td_api::messageVoiceNote &>(
			*message.content_).caption_.get();
	case td_api::messageAnimation::ID:
		return static_cast<const td_api::messageAnimation &>(
			*message.content_).caption_.get();
	default:
		return nullptr;
	}
}

} /* namespace tgloggerd */
