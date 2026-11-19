// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Ammar Faizi <ammarfaizi2@gnuweeb.org>
 */
/*
 * Mapping of TDLib message content onto the models stored in the database,
 * kept apart from TDLib.cpp so that it can be tested without a TDLib client.
 */
#include <tgloggerd/MessageExtract.hpp>

#include <td/telegram/td_api.hpp>
#include <td/telegram/td_api_json.h>
#include <td/utils/JsonBuilder.h>

#include <nlohmann/json.hpp>

#include <cstdio>
#include <cstring>
#include <exception>
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
 * nullopt for a service class described generically by the caller.
 *
 * @sender_uid is the user who sent the message, 0 if unknown or not a user;
 * it tells a self-action (joined, left) from one done to someone else.
 */
std::optional<std::string>
describe_service_message(const td_api::MessageContent &content,
			 int64_t sender_uid, std::string &service_type)
{
	switch (content.get_id()) {
	case td_api::messageChatAddMembers::ID: {
		auto &c = static_cast<const td_api::messageChatAddMembers &>(content);
		service_type = "chat_add_members";
		/* Exactly one added member, equal to the sender: a self-join. */
		if (c.member_user_ids_.size() == 1 &&
		    c.member_user_ids_[0] == sender_uid)
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
		if (c.user_id_ == sender_uid)
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
	case td_api::messageForumTopicEdited::ID: {
		auto &c = static_cast<const td_api::messageForumTopicEdited &>(content);
		service_type = "forum_topic_edited";
		if (!c.name_.empty())
			return "renamed a topic to \"" + c.name_ + "\"";
		if (c.edit_icon_custom_emoji_id_)
			return "changed a topic's icon";
		return "edited a topic";
	}
	case td_api::messageForumTopicIsClosedToggled::ID:
		service_type = "forum_topic_is_closed_toggled";
		return static_cast<const td_api::messageForumTopicIsClosedToggled &>(
			content).is_closed_ ? "closed the topic" : "reopened the topic";
	case td_api::messageForumTopicIsHiddenToggled::ID:
		service_type = "forum_topic_is_hidden_toggled";
		return static_cast<const td_api::messageForumTopicIsHiddenToggled &>(
			content).is_hidden_ ? "hid the topic" : "unhid the topic";
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
	case td_api::messageChatBoost::ID: {
		auto &c = static_cast<const td_api::messageChatBoost &>(content);
		service_type = "chat_boost";
		if (c.boost_count_ > 1)
			return "boosted the group " +
				std::to_string(c.boost_count_) + " times";
		return "boosted the group";
	}
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
		service_type = "payment_successful";
		return "made a payment";
	case td_api::messagePaymentSuccessfulBot::ID:
		service_type = "payment_successful_bot";
		return "made a payment";
	case td_api::messageCustomServiceAction::ID: {
		auto &c = static_cast<const td_api::messageCustomServiceAction &>(content);
		service_type = "custom_service_action";
		return c.text_;
	}
	case td_api::messageChatJoinFromCommunity::ID:
		service_type = "chat_join_from_community";
		return "joined the group from a community";
	case td_api::messageChatAddedToCommunity::ID:
		service_type = "chat_added_to_community";
		return "added the group to a community";
	case td_api::messageChatRemovedFromCommunity::ID:
		service_type = "chat_removed_from_community";
		return "removed the group from its community";
	case td_api::messageChatOwnerLeft::ID:
		service_type = "chat_owner_left";
		return "left the group and handed over ownership";
	case td_api::messageChatOwnerChanged::ID:
		service_type = "chat_owner_changed";
		return "transferred ownership of the group";
	case td_api::messageChatHasProtectedContentToggled::ID:
		service_type = "chat_has_protected_content_toggled";
		return static_cast<const td_api::messageChatHasProtectedContentToggled &>(
			content).new_has_protected_content_
			? "turned on content protection"
			: "turned off content protection";
	case td_api::messageChatHasProtectedContentDisableRequested::ID:
		service_type = "chat_has_protected_content_disable_requested";
		return "asked to turn off content protection";
	case td_api::messageSuggestProfilePhoto::ID:
		service_type = "suggest_profile_photo";
		return "suggested a profile photo";
	case td_api::messageSuggestBirthdate::ID:
		service_type = "suggest_birthdate";
		return "suggested a birthdate";
	case td_api::messageGameScore::ID:
		service_type = "game_score";
		return "scored " + std::to_string(
			static_cast<const td_api::messageGameScore &>(content).score_) +
			" in a game";
	case td_api::messageManagedBotCreated::ID:
		service_type = "managed_bot_created";
		return "created a bot";
	case td_api::messagePaymentRefunded::ID:
		service_type = "payment_refunded";
		return "refunded a payment";
	case td_api::messagePremiumGiftCode::ID:
		service_type = "premium_gift_code";
		return "sent a Telegram Premium gift code";
	case td_api::messageGiveawayCreated::ID:
		service_type = "giveaway_created";
		return "created a giveaway";
	case td_api::messageGiveawayCompleted::ID:
		service_type = "giveaway_completed";
		return "the giveaway was completed";
	case td_api::messageGiveawayWinners::ID:
		service_type = "giveaway_winners";
		return "announced the giveaway winners";
	case td_api::messageGiftedStars::ID:
		service_type = "gifted_stars";
		return "gifted Telegram Stars";
	case td_api::messageGiftedGrams::ID:
		service_type = "gifted_grams";
		return "sent a gift of grams";
	case td_api::messageGiveawayPrizeStars::ID:
		service_type = "giveaway_prize_stars";
		return "won Telegram Stars in a giveaway";
	case td_api::messageTonWalletTransfer::ID:
		service_type = "ton_wallet_transfer";
		return "made a TON wallet transfer";
	case td_api::messageTonConnectRequest::ID:
		service_type = "ton_connect_request";
		return "received a TON Connect request";
	case td_api::messageGift::ID:
		service_type = "gift";
		return "sent a gift";
	case td_api::messageUpgradedGift::ID: {
		auto &c = static_cast<const td_api::messageUpgradedGift &>(content);
		service_type = "upgraded_gift";
		if (c.origin_ &&
		    c.origin_->get_id() == td_api::upgradedGiftOriginUpgrade::ID)
			return "upgraded a gift";
		return "sent an upgraded gift";
	}
	case td_api::messageRefundedUpgradedGift::ID:
		service_type = "refunded_upgraded_gift";
		return "the gift was refunded";
	case td_api::messageUpgradedGiftPurchaseOffer::ID:
		service_type = "upgraded_gift_purchase_offer";
		return "offered to buy a gift";
	case td_api::messageUpgradedGiftPurchaseOfferRejected::ID:
		service_type = "upgraded_gift_purchase_offer_rejected";
		return static_cast<const td_api::messageUpgradedGiftPurchaseOfferRejected &>(
			content).was_expired_
			? "the offer to buy a gift expired"
			: "rejected an offer to buy a gift";
	case td_api::messagePaidMessagesRefunded::ID:
		service_type = "paid_messages_refunded";
		return "refunded paid messages";
	case td_api::messagePaidMessagePriceChanged::ID: {
		auto n = static_cast<const td_api::messagePaidMessagePriceChanged &>(
			content).paid_message_star_count_;
		service_type = "paid_message_price_changed";
		if (n > 0)
			return "set the price of a message to " +
				std::to_string(n) + " Telegram Stars";
		return "made messages free to send";
	}
	case td_api::messageDirectMessagePriceChanged::ID: {
		auto &c = static_cast<const td_api::messageDirectMessagePriceChanged &>(
			content);
		service_type = "direct_message_price_changed";
		if (!c.is_enabled_)
			return "turned off direct messages";
		if (c.paid_message_star_count_ > 0)
			return "set the price of a direct message to " +
				std::to_string(c.paid_message_star_count_) +
				" Telegram Stars";
		return "turned on direct messages";
	}
	case td_api::messageUnpaidMessagesAllowedByLink::ID:
		service_type = "unpaid_messages_allowed_by_link";
		return "may now send messages without paying, through a link";
	case td_api::messageChecklistTasksDone::ID: {
		auto &c = static_cast<const td_api::messageChecklistTasksDone &>(
			content);
		service_type = "checklist_tasks_done";
		if (c.marked_as_done_task_ids_.empty())
			return "marked checklist tasks as not done";
		return "marked checklist tasks as done";
	}
	case td_api::messageChecklistTasksAdded::ID:
		service_type = "checklist_tasks_added";
		return "added tasks to a checklist";
	case td_api::messagePollOptionAdded::ID:
		service_type = "poll_option_added";
		return "added an option to a poll";
	case td_api::messagePollOptionDeleted::ID:
		service_type = "poll_option_deleted";
		return "removed an option from a poll";
	case td_api::messageSuggestedPostApprovalFailed::ID:
		service_type = "suggested_post_approval_failed";
		return "the suggested post could not be approved";
	case td_api::messageSuggestedPostApproved::ID:
		service_type = "suggested_post_approved";
		return "approved a suggested post";
	case td_api::messageSuggestedPostDeclined::ID:
		service_type = "suggested_post_declined";
		return "declined a suggested post";
	case td_api::messageSuggestedPostPaid::ID:
		service_type = "suggested_post_paid";
		return "the suggested post was paid for";
	case td_api::messageSuggestedPostRefunded::ID:
		service_type = "suggested_post_refunded";
		return "the suggested post was refunded";
	case td_api::messageUsersShared::ID: {
		auto n = static_cast<const td_api::messageUsersShared &>(
			content).users_.size();
		service_type = "users_shared";
		if (n > 1)
			return "shared " + std::to_string(n) +
				" users with the bot";
		return "shared a user with the bot";
	}
	case td_api::messageChatShared::ID:
		service_type = "chat_shared";
		return "shared a chat with the bot";
	case td_api::messageBotWriteAccessAllowed::ID:
		service_type = "bot_write_access_allowed";
		return "allowed the bot to send messages";
	case td_api::messageWebAppDataSent::ID:
		service_type = "web_app_data_sent";
		return "sent data to the bot";
	case td_api::messageWebAppDataReceived::ID:
		service_type = "web_app_data_received";
		return "sent data from a web app";
	case td_api::messagePassportDataSent::ID:
		service_type = "passport_data_sent";
		return "shared Telegram Passport data";
	case td_api::messagePassportDataReceived::ID:
		service_type = "passport_data_received";
		return "received Telegram Passport data";
	case td_api::messageProximityAlertTriggered::ID: {
		auto d = static_cast<const td_api::messageProximityAlertTriggered &>(
			content).distance_;
		service_type = "proximity_alert_triggered";
		return "came within " + std::to_string(d) + " metres";
	}
	default:
		return std::nullopt;
	}
}


/*
 * The text a content class carries as its message text or caption, with
 * its entities, or nullptr. Service classes that carry a text of their own
 * (a gift's message) keep it in content_extra instead: the text column of a
 * service message is its description.
 */
const td_api::formattedText *
content_formatted_text(const td_api::MessageContent &c)
{
	switch (c.get_id()) {
	case td_api::messageText::ID:
		return static_cast<const td_api::messageText &>(c).text_.get();
	case td_api::messagePhoto::ID:
		return static_cast<const td_api::messagePhoto &>(c).caption_.get();
	case td_api::messageVideo::ID:
		return static_cast<const td_api::messageVideo &>(c).caption_.get();
	case td_api::messageDocument::ID:
		return static_cast<const td_api::messageDocument &>(c).caption_.get();
	case td_api::messageAudio::ID:
		return static_cast<const td_api::messageAudio &>(c).caption_.get();
	case td_api::messageVoiceNote::ID:
		return static_cast<const td_api::messageVoiceNote &>(c).caption_.get();
	case td_api::messageAnimation::ID:
		return static_cast<const td_api::messageAnimation &>(c).caption_.get();
	case td_api::messagePaidMedia::ID:
		return static_cast<const td_api::messagePaidMedia &>(c).caption_.get();
	case td_api::messageWidget::ID:
		return static_cast<const td_api::messageWidget &>(c).caption_.get();
	case td_api::messagePoll::ID:
		return static_cast<const td_api::messagePoll &>(c).description_.get();
	case td_api::messageInvoice::ID:
		return static_cast<const td_api::messageInvoice &>(c)
			.paid_media_caption_.get();
	case td_api::messageGame::ID: {
		auto &g = static_cast<const td_api::messageGame &>(c);
		return g.game_ ? g.game_->text_.get() : nullptr;
	}
	case td_api::messageChecklist::ID: {
		auto &l = static_cast<const td_api::messageChecklist &>(c);
		return l.list_ ? l.list_->title_.get() : nullptr;
	}
	default:
		return nullptr;
	}
}

models::MessageContentType content_kind(int32_t id)
{
	switch (id) {
#define TGLOGGERD_KIND_CASE(cls, kind) \
	case td_api::cls::ID: return models::MessageContentType::kind;
	TGLOGGERD_MESSAGE_CONTENT_CLASSES(TGLOGGERD_KIND_CASE)
#undef TGLOGGERD_KIND_CASE
	default:
		return models::MessageContentType::Unknown;
	}
}

/*
 * "messageChatOwnerChanged" -> "chat_owner_changed": the service_type of a
 * service class without a name of its own (decision
 * 2026-11-18-msg-service-type-names).
 */
std::string service_type_name(const std::string &cls)
{
	std::string s = cls.rfind("message", 0) == 0 ? cls.substr(7) : cls;
	std::string out;
	for (char ch : s) {
		if (ch >= 'A' && ch <= 'Z') {
			if (!out.empty())
				out += '_';
			out += (char)(ch - 'A' + 'a');
		} else {
			out += ch;
		}
	}
	return out.empty() ? std::string("unknown") : out;
}

/* "chat_owner_changed" -> "chat owner changed": a description for a service
 * message the describer does not know yet; msg-service-detail replaces it. */
std::string generic_service_description(const std::string &service_type)
{
	std::string out = service_type;
	for (char &ch : out)
		if (ch == '_')
			ch = ' ';
	return out;
}

/* Remove what only makes sense to this TDLib session: local file state
 * (paths, download progress) and the account's own viewed/listened marks. */
void strip_local_state(nlohmann::json &j)
{
	if (j.is_object()) {
		auto t = j.find("@type");
		if (t != j.end() && t->is_string() && *t == "file")
			j.erase("local");
		j.erase("is_viewed");
		j.erase("is_listened");
		for (auto &kv : j.items())
			strip_local_state(kv.value());
	} else if (j.is_array()) {
		for (auto &v : j)
			strip_local_state(v);
	}
}

/* Bigger contents (a rich message or a widget with a large page) keep only
 * their class, so one message cannot fill a JSON column with megabytes. */
constexpr size_t kMaxTdlibJson = 256 * 1024;

std::optional<nlohmann::json> tdlib_json(const td_api::MessageContent &c)
{
	try {
		std::string raw = td::json_encode<std::string>(td::ToJson(c));
		return nlohmann::json::parse(raw);
	} catch (const std::exception &) {
		return std::nullopt;
	}
}

std::optional<std::string>
tdlib_class_from_json(const td_api::MessageContent &c)
{
	auto j = tdlib_json(c);
	if (j && j->is_object() && j->contains("@type") &&
	    (*j)["@type"].is_string())
		return (*j)["@type"].get<std::string>();
	return std::nullopt;
}

/* {"tdlib": <the content as TDLib's JSON, local state removed>}, or nullopt
 * when the content carries nothing beyond its class. */
std::optional<std::string>
tdlib_extra_json(const td_api::MessageContent &c)
{
	auto j = tdlib_json(c);
	if (!j || !j->is_object())
		return std::nullopt;
	strip_local_state(*j);
	if (j->size() <= 1)
		return std::nullopt;
	std::string body = j->dump();
	if (body.size() > kMaxTdlibJson) {
		nlohmann::json small = {
			{ "@type", (*j)["@type"] },
		};
		nlohmann::json out = {
			{ "tdlib", small },
			{ "tdlib_omitted_bytes", body.size() },
		};
		return out.dump();
	}
	return std::string("{\"tdlib\":") + body + "}";
}

/*
 * The plain text of a rich text: every run of text it shows, the
 * alternative text of a custom emoji, a formula's source and a button's
 * label. Icons and anchors show no text.
 */
void rich_text_append(const td_api::RichText *t, std::string &out)
{
	if (!t)
		return;
	switch (t->get_id()) {
	case td_api::richTextPlain::ID:
		out += static_cast<const td_api::richTextPlain &>(*t).text_;
		break;
#define TGLOGGERD_RICH_WRAP(cls) \
	case td_api::cls::ID: \
		rich_text_append(static_cast<const td_api::cls &>(*t).text_.get(), \
				 out); \
		break;
	TGLOGGERD_RICH_WRAP(richTextBold)
	TGLOGGERD_RICH_WRAP(richTextItalic)
	TGLOGGERD_RICH_WRAP(richTextUnderline)
	TGLOGGERD_RICH_WRAP(richTextStrikethrough)
	TGLOGGERD_RICH_WRAP(richTextSpoiler)
	TGLOGGERD_RICH_WRAP(richTextSubscript)
	TGLOGGERD_RICH_WRAP(richTextSuperscript)
	TGLOGGERD_RICH_WRAP(richTextMarked)
	TGLOGGERD_RICH_WRAP(richTextDateTime)
	TGLOGGERD_RICH_WRAP(richTextMention)
	TGLOGGERD_RICH_WRAP(richTextHashtag)
	TGLOGGERD_RICH_WRAP(richTextCashtag)
	TGLOGGERD_RICH_WRAP(richTextBankCardNumber)
	TGLOGGERD_RICH_WRAP(richTextTonAddress)
	TGLOGGERD_RICH_WRAP(richTextBotCommand)
	TGLOGGERD_RICH_WRAP(richTextFixed)
	TGLOGGERD_RICH_WRAP(richTextMentionName)
	TGLOGGERD_RICH_WRAP(richTextUrl)
	TGLOGGERD_RICH_WRAP(richTextEmailAddress)
	TGLOGGERD_RICH_WRAP(richTextPhoneNumber)
	TGLOGGERD_RICH_WRAP(richTextDiff)
	TGLOGGERD_RICH_WRAP(richTextReference)
	TGLOGGERD_RICH_WRAP(richTextReferenceLink)
	TGLOGGERD_RICH_WRAP(richTextAnchorLink)
#undef TGLOGGERD_RICH_WRAP
	case td_api::richTextCustomEmoji::ID:
		out += static_cast<const td_api::richTextCustomEmoji &>(*t)
			       .alternative_text_;
		break;
	case td_api::richTextMathematicalExpression::ID:
		out += static_cast<const td_api::richTextMathematicalExpression &>(
			       *t).expression_;
		break;
	case td_api::richTextButton::ID: {
		auto &b = static_cast<const td_api::richTextButton &>(*t);
		if (b.button_)
			rich_text_append(b.button_->text_.get(), out);
		break;
	}
	case td_api::richTexts::ID:
		for (const auto &x : static_cast<const td_api::richTexts &>(*t).texts_)
			rich_text_append(x.get(), out);
		break;
	default:
		break;
	}
}

/* Append @s to @lines unless it is empty. */
void add_line(std::vector<std::string> &lines, std::string s)
{
	if (!s.empty())
		lines.push_back(std::move(s));
}

void add_rich_line(std::vector<std::string> &lines, const td_api::RichText *t)
{
	std::string s;
	rich_text_append(t, s);
	add_line(lines, std::move(s));
}

void add_caption_lines(std::vector<std::string> &lines,
		       const td_api::pageBlockCaption *c)
{
	if (!c)
		return;
	add_rich_line(lines, c->text_.get());
	add_rich_line(lines, c->credit_.get());
}

void page_blocks_text(
	const std::vector<td_api::object_ptr<td_api::PageBlock>> &blocks,
	std::vector<std::string> &lines);

/*
 * The lines of text one page block shows, in reading order: headings,
 * paragraphs, quotes and their credit, captions, list items (after their
 * label), table cells (a row on one line, its cells apart by " | "),
 * the header and titles of related articles, button labels.
 */
void page_block_text(const td_api::PageBlock *b, std::vector<std::string> &lines)
{
	if (!b)
		return;
	switch (b->get_id()) {
#define TGLOGGERD_BLOCK_TEXT(cls, field) \
	case td_api::cls::ID: \
		add_rich_line(lines, \
			      static_cast<const td_api::cls &>(*b).field.get()); \
		break;
	TGLOGGERD_BLOCK_TEXT(pageBlockTitle, title_)
	TGLOGGERD_BLOCK_TEXT(pageBlockSubtitle, subtitle_)
	TGLOGGERD_BLOCK_TEXT(pageBlockAuthorDate, author_)
	TGLOGGERD_BLOCK_TEXT(pageBlockHeader, header_)
	TGLOGGERD_BLOCK_TEXT(pageBlockSubheader, subheader_)
	TGLOGGERD_BLOCK_TEXT(pageBlockSectionHeading, text_)
	TGLOGGERD_BLOCK_TEXT(pageBlockKicker, kicker_)
	TGLOGGERD_BLOCK_TEXT(pageBlockParagraph, text_)
	TGLOGGERD_BLOCK_TEXT(pageBlockPreformatted, text_)
	TGLOGGERD_BLOCK_TEXT(pageBlockFooter, footer_)
	TGLOGGERD_BLOCK_TEXT(pageBlockThinking, text_)
#undef TGLOGGERD_BLOCK_TEXT
#define TGLOGGERD_BLOCK_CAPTION(cls) \
	case td_api::cls::ID: \
		add_caption_lines(lines, \
				  static_cast<const td_api::cls &>(*b).caption_.get()); \
		break;
	TGLOGGERD_BLOCK_CAPTION(pageBlockAnimation)
	TGLOGGERD_BLOCK_CAPTION(pageBlockAudio)
	TGLOGGERD_BLOCK_CAPTION(pageBlockDocument)
	TGLOGGERD_BLOCK_CAPTION(pageBlockPhoto)
	TGLOGGERD_BLOCK_CAPTION(pageBlockVideo)
	TGLOGGERD_BLOCK_CAPTION(pageBlockVoiceNote)
	TGLOGGERD_BLOCK_CAPTION(pageBlockEmbedded)
	TGLOGGERD_BLOCK_CAPTION(pageBlockMap)
#undef TGLOGGERD_BLOCK_CAPTION
	case td_api::pageBlockMathematicalExpression::ID:
		add_line(lines, static_cast<const td_api::pageBlockMathematicalExpression &>(
					*b).expression_);
		break;
	case td_api::pageBlockList::ID:
		for (const auto &it : static_cast<const td_api::pageBlockList &>(
					      *b).items_) {
			if (!it)
				continue;
			std::vector<std::string> sub;
			page_blocks_text(it->blocks_, sub);
			if (!it->label_.empty()) {
				if (sub.empty())
					sub.push_back(it->label_);
				else
					sub[0] = it->label_ + " " + sub[0];
			}
			for (auto &s : sub)
				lines.push_back(std::move(s));
		}
		break;
	case td_api::pageBlockBlockQuote::ID: {
		auto &q = static_cast<const td_api::pageBlockBlockQuote &>(*b);
		page_blocks_text(q.blocks_, lines);
		add_rich_line(lines, q.credit_.get());
		break;
	}
	case td_api::pageBlockExpandableBlockQuote::ID: {
		auto &q = static_cast<const td_api::pageBlockExpandableBlockQuote &>(*b);
		add_rich_line(lines, q.text_.get());
		add_rich_line(lines, q.credit_.get());
		break;
	}
	case td_api::pageBlockPullQuote::ID: {
		auto &q = static_cast<const td_api::pageBlockPullQuote &>(*b);
		add_rich_line(lines, q.text_.get());
		add_rich_line(lines, q.credit_.get());
		break;
	}
	case td_api::pageBlockCover::ID:
		page_block_text(static_cast<const td_api::pageBlockCover &>(*b)
					.cover_.get(), lines);
		break;
	case td_api::pageBlockEmbeddedPost::ID: {
		auto &p = static_cast<const td_api::pageBlockEmbeddedPost &>(*b);
		page_blocks_text(p.blocks_, lines);
		add_caption_lines(lines, p.caption_.get());
		break;
	}
	case td_api::pageBlockCollage::ID: {
		auto &c = static_cast<const td_api::pageBlockCollage &>(*b);
		page_blocks_text(c.blocks_, lines);
		add_caption_lines(lines, c.caption_.get());
		break;
	}
	case td_api::pageBlockSlideshow::ID: {
		auto &c = static_cast<const td_api::pageBlockSlideshow &>(*b);
		page_blocks_text(c.blocks_, lines);
		add_caption_lines(lines, c.caption_.get());
		break;
	}
	case td_api::pageBlockChatLink::ID:
		add_line(lines, static_cast<const td_api::pageBlockChatLink &>(
					*b).title_);
		break;
	case td_api::pageBlockTable::ID: {
		auto &t = static_cast<const td_api::pageBlockTable &>(*b);
		add_rich_line(lines, t.caption_.get());
		for (const auto &row : t.cells_) {
			std::string line;
			for (const auto &cell : row) {
				if (!cell)
					continue;
				std::string s;
				rich_text_append(cell->text_.get(), s);
				if (s.empty())
					continue;
				if (!line.empty())
					line += " | ";
				line += s;
			}
			add_line(lines, std::move(line));
		}
		break;
	}
	case td_api::pageBlockDetails::ID: {
		auto &d = static_cast<const td_api::pageBlockDetails &>(*b);
		add_rich_line(lines, d.header_.get());
		page_blocks_text(d.blocks_, lines);
		break;
	}
	case td_api::pageBlockRelatedArticles::ID: {
		auto &r = static_cast<const td_api::pageBlockRelatedArticles &>(*b);
		add_rich_line(lines, r.header_.get());
		for (const auto &a : r.articles_) {
			if (!a)
				continue;
			add_line(lines, a->title_);
			add_line(lines, a->description_);
		}
		break;
	}
	case td_api::pageBlockButtonRow::ID:
		for (const auto &btn : static_cast<const td_api::pageBlockButtonRow &>(
					       *b).buttons_)
			if (btn)
				add_rich_line(lines, btn->text_.get());
		break;
	default:
		/* Divider, anchor, unsupported: no text. */
		break;
	}
}

void page_blocks_text(
	const std::vector<td_api::object_ptr<td_api::PageBlock>> &blocks,
	std::vector<std::string> &lines)
{
	for (const auto &b : blocks)
		page_block_text(b.get(), lines);
}

/* telegram_message_files.position is a SMALLINT. */
constexpr size_t kMaxRichFiles = 32767;

void page_blocks_files(
	const std::vector<td_api::object_ptr<td_api::PageBlock>> &blocks,
	std::vector<ExtraFile> &out);

/*
 * The media of one page block, appended to @out in order of appearance
 * as role "rich", position its index in @out: a photo (largest size), a
 * video, an animation, an audio file, a document or a voice note, an
 * embedded page's poster, an embedded post's author photo and the
 * photos of related articles; the blocks inside a cover, a collage, a
 * slideshow, a quote, a list, a collapsible block or an embedded post.
 * A chat link's photo (the chat's own) and the icons inside rich text
 * are not listed.
 */
void page_block_files(const td_api::PageBlock *b, std::vector<ExtraFile> &out)
{
	auto add = [&out](const td_api::file *f, const char *category,
			  std::string name) {
		if (f && out.size() < kMaxRichFiles)
			out.push_back(ExtraFile{ f, category, std::move(name),
						 "rich", (int)out.size() });
	};
	if (!b)
		return;
	switch (b->get_id()) {
	case td_api::pageBlockPhoto::ID:
		add(photo_best_size(static_cast<const td_api::pageBlockPhoto &>(
					    *b).photo_.get()), "photo", "");
		break;
	case td_api::pageBlockVideo::ID: {
		auto &v = static_cast<const td_api::pageBlockVideo &>(*b);
		if (v.video_)
			add(v.video_->video_.get(), "video", v.video_->file_name_);
		break;
	}
	case td_api::pageBlockAnimation::ID: {
		auto &a = static_cast<const td_api::pageBlockAnimation &>(*b);
		if (a.animation_)
			add(a.animation_->animation_.get(), "animation",
			    a.animation_->file_name_);
		break;
	}
	case td_api::pageBlockAudio::ID: {
		auto &a = static_cast<const td_api::pageBlockAudio &>(*b);
		if (a.audio_)
			add(a.audio_->audio_.get(), "audio", a.audio_->file_name_);
		break;
	}
	case td_api::pageBlockDocument::ID: {
		auto &d = static_cast<const td_api::pageBlockDocument &>(*b);
		if (d.document_)
			add(d.document_->document_.get(), "document",
			    d.document_->file_name_);
		break;
	}
	case td_api::pageBlockVoiceNote::ID: {
		auto &v = static_cast<const td_api::pageBlockVoiceNote &>(*b);
		if (v.voice_note_)
			add(v.voice_note_->voice_.get(), "voice", "");
		break;
	}
	case td_api::pageBlockEmbedded::ID:
		add(photo_best_size(static_cast<const td_api::pageBlockEmbedded &>(
					    *b).poster_photo_.get()), "photo", "");
		break;
	case td_api::pageBlockEmbeddedPost::ID: {
		auto &p = static_cast<const td_api::pageBlockEmbeddedPost &>(*b);
		add(photo_best_size(p.author_photo_.get()), "photo", "");
		page_blocks_files(p.blocks_, out);
		break;
	}
	case td_api::pageBlockRelatedArticles::ID:
		for (const auto &a : static_cast<const td_api::pageBlockRelatedArticles &>(
					     *b).articles_)
			if (a)
				add(photo_best_size(a->photo_.get()), "photo", "");
		break;
	case td_api::pageBlockCover::ID:
		page_block_files(static_cast<const td_api::pageBlockCover &>(*b)
					 .cover_.get(), out);
		break;
	case td_api::pageBlockCollage::ID:
		page_blocks_files(static_cast<const td_api::pageBlockCollage &>(
					  *b).blocks_, out);
		break;
	case td_api::pageBlockSlideshow::ID:
		page_blocks_files(static_cast<const td_api::pageBlockSlideshow &>(
					  *b).blocks_, out);
		break;
	case td_api::pageBlockBlockQuote::ID:
		page_blocks_files(static_cast<const td_api::pageBlockBlockQuote &>(
					  *b).blocks_, out);
		break;
	case td_api::pageBlockDetails::ID:
		page_blocks_files(static_cast<const td_api::pageBlockDetails &>(
					  *b).blocks_, out);
		break;
	case td_api::pageBlockList::ID:
		for (const auto &it : static_cast<const td_api::pageBlockList &>(
					      *b).items_)
			if (it)
				page_blocks_files(it->blocks_, out);
		break;
	default:
		break;
	}
}

void page_blocks_files(
	const std::vector<td_api::object_ptr<td_api::PageBlock>> &blocks,
	std::vector<ExtraFile> &out)
{
	for (const auto &b : blocks)
		page_block_files(b.get(), out);
}

/*
 * A rich message's content_extra: the curated {"blocks": <top-level block
 * count>, "is_rtl": <bool>} beside TDLib's JSON, which stays the render
 * source (decision 2026-11-19-msg-rich-raw-json). The curated keys come from
 * the live object, so a message whose JSON is over the cap still has them.
 */
std::string rich_extra_json(const td_api::messageRichMessage &c)
{
	nlohmann::json cur = {
		{ "blocks", c.message_ ? c.message_->blocks_.size() : 0 },
		{ "is_rtl", c.message_ ? c.message_->is_rtl_ : false },
	};
	std::string out = cur.dump();
	std::optional<std::string> raw = tdlib_extra_json(c);
	if (raw && raw->size() > 2 && (*raw)[0] == '{') {
		out.pop_back();
		out += ',';
		out.append(*raw, 1, std::string::npos);
	}
	return out;
}


/* Standard base64 of raw bytes (TDLib's `bytes` fields). */
std::string base64(const std::string &in)
{
	static const char tbl[] =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	std::string out;
	out.reserve((in.size() + 2) / 3 * 4);
	size_t i = 0;
	for (; i + 2 < in.size(); i += 3) {
		uint32_t v = ((uint8_t)in[i] << 16) | ((uint8_t)in[i + 1] << 8) |
			     (uint8_t)in[i + 2];
		out += tbl[(v >> 18) & 63];
		out += tbl[(v >> 12) & 63];
		out += tbl[(v >> 6) & 63];
		out += tbl[v & 63];
	}
	if (i < in.size()) {
		uint32_t v = (uint8_t)in[i] << 16;
		if (i + 1 < in.size())
			v |= (uint8_t)in[i + 1] << 8;
		out += tbl[(v >> 18) & 63];
		out += tbl[(v >> 12) & 63];
		out += i + 1 < in.size() ? tbl[(v >> 6) & 63] : '=';
		out += '=';
	}
	return out;
}

/*
 * Curated media keys (docs/design/message-types.md section 4.2.1): a key is
 * set only when it applies, so a flag only when true, a number only when
 * non-zero and a string only when non-empty.
 */
void put_flag(nlohmann::json &j, const char *key, bool v)
{
	if (v)
		j[key] = true;
}

void put_num(nlohmann::json &j, const char *key, int64_t v)
{
	if (v != 0)
		j[key] = v;
}

void put_str(nlohmann::json &j, const char *key, const std::string &v)
{
	if (!v.empty())
		j[key] = v;
}

/* A 64-bit id as a string, as TDLib's JSON writes it. */
void put_id(nlohmann::json &j, const char *key, int64_t v)
{
	if (v != 0)
		j[key] = std::to_string(v);
}

void put_minithumbnail(nlohmann::json &j, const td_api::minithumbnail *m)
{
	if (!m || m->data_.empty())
		return;
	j["minithumbnail"] = {
		{ "width", m->width_ },
		{ "height", m->height_ },
		{ "data", base64(m->data_) },
	};
}

void put_transcript(nlohmann::json &j, const td_api::SpeechRecognitionResult *r)
{
	if (r && r->get_id() == td_api::speechRecognitionResultText::ID)
		put_str(j, "transcript",
			static_cast<const td_api::speechRecognitionResultText &>(
				*r).text_);
}

void put_video(nlohmann::json &j, const td_api::video *v)
{
	if (!v)
		return;
	put_num(j, "width", v->width_);
	put_num(j, "height", v->height_);
	put_num(j, "duration", v->duration_);
	put_str(j, "mime", v->mime_type_);
	put_flag(j, "has_stickers", v->has_stickers_);
	put_flag(j, "streaming", v->supports_streaming_);
	put_minithumbnail(j, v->minithumbnail_.get());
}

/* The largest size of a photo, which is the one stored. */
const td_api::photoSize *largest_size(const td_api::photo *p)
{
	const td_api::photoSize *best = nullptr;
	if (!p)
		return nullptr;
	for (const auto &sz : p->sizes_)
		if (sz && (!best || (int64_t)sz->width_ * sz->height_ >
				    (int64_t)best->width_ * best->height_))
			best = sz.get();
	return best;
}

void put_photo(nlohmann::json &j, const td_api::photo *p)
{
	if (!p)
		return;
	if (auto *sz = largest_size(p)) {
		put_num(j, "width", sz->width_);
		put_num(j, "height", sz->height_);
	}
	put_flag(j, "has_stickers", p->has_stickers_);
	put_minithumbnail(j, p->minithumbnail_.get());
}

const char *sticker_format_name(const td_api::StickerFormat *f)
{
	if (!f)
		return "";
	switch (f->get_id()) {
	case td_api::stickerFormatWebp::ID: return "webp";
	case td_api::stickerFormatTgs::ID:  return "tgs";
	case td_api::stickerFormatWebm::ID: return "webm";
	default:                            return "";
	}
}

/* The curated content_extra of a media class, or nullopt when @c is not a
 * media class (@is_media false) or carries nothing to keep. */
std::optional<std::string>
media_extra_json(const td_api::MessageContent &c, bool &is_media)
{
	nlohmann::json j = nlohmann::json::object();
	is_media = true;

	switch (c.get_id()) {
	case td_api::messagePhoto::ID: {
		auto &m = static_cast<const td_api::messagePhoto &>(c);
		put_photo(j, m.photo_.get());
		put_flag(j, "spoiler", m.has_spoiler_);
		put_flag(j, "caption_above", m.show_caption_above_media_);
		put_flag(j, "secret", m.is_secret_);
		if (m.video_) {
			nlohmann::json live = nlohmann::json::object();
			put_num(live, "width", m.video_->width_);
			put_num(live, "height", m.video_->height_);
			put_num(live, "duration", m.video_->duration_);
			put_str(live, "mime", m.video_->mime_type_);
			j["live_photo"] = live;
		}
		break;
	}
	case td_api::messageVideo::ID: {
		auto &m = static_cast<const td_api::messageVideo &>(c);
		put_video(j, m.video_.get());
		put_flag(j, "spoiler", m.has_spoiler_);
		put_flag(j, "caption_above", m.show_caption_above_media_);
		put_flag(j, "secret", m.is_secret_);
		put_num(j, "start_timestamp", m.start_timestamp_);
		put_flag(j, "cover", m.cover_ != nullptr);
		nlohmann::json q = nlohmann::json::array();
		for (const auto &a : m.alternative_videos_) {
			if (!a)
				continue;
			nlohmann::json e = nlohmann::json::object();
			put_num(e, "width", a->width_);
			put_num(e, "height", a->height_);
			put_str(e, "codec", a->codec_);
			q.push_back(e);
		}
		if (!q.empty())
			j["qualities"] = q;
		break;
	}
	case td_api::messageAnimation::ID: {
		auto &m = static_cast<const td_api::messageAnimation &>(c);
		if (auto *a = m.animation_.get()) {
			put_num(j, "width", a->width_);
			put_num(j, "height", a->height_);
			put_num(j, "duration", a->duration_);
			put_str(j, "mime", a->mime_type_);
			put_flag(j, "has_stickers", a->has_stickers_);
			put_minithumbnail(j, a->minithumbnail_.get());
		}
		put_flag(j, "spoiler", m.has_spoiler_);
		put_flag(j, "caption_above", m.show_caption_above_media_);
		put_flag(j, "secret", m.is_secret_);
		break;
	}
	case td_api::messageAudio::ID: {
		auto &m = static_cast<const td_api::messageAudio &>(c);
		if (auto *a = m.audio_.get()) {
			put_num(j, "duration", a->duration_);
			put_str(j, "mime", a->mime_type_);
			put_str(j, "title", a->title_);
			put_str(j, "performer", a->performer_);
			put_minithumbnail(j,
				a->album_cover_minithumbnail_.get());
			put_flag(j, "album_cover",
				 a->album_cover_thumbnail_ != nullptr);
		}
		break;
	}
	case td_api::messageDocument::ID: {
		auto &m = static_cast<const td_api::messageDocument &>(c);
		if (auto *d = m.document_.get()) {
			put_str(j, "mime", d->mime_type_);
			put_minithumbnail(j, d->minithumbnail_.get());
		}
		break;
	}
	case td_api::messageVoiceNote::ID: {
		auto &m = static_cast<const td_api::messageVoiceNote &>(c);
		if (auto *v = m.voice_note_.get()) {
			put_num(j, "duration", v->duration_);
			put_str(j, "mime", v->mime_type_);
			if (!v->waveform_.empty())
				j["waveform"] = base64(v->waveform_);
			put_transcript(j, v->speech_recognition_result_.get());
		}
		break;
	}
	case td_api::messageVideoNote::ID: {
		auto &m = static_cast<const td_api::messageVideoNote &>(c);
		if (auto *v = m.video_note_.get()) {
			put_num(j, "duration", v->duration_);
			put_num(j, "length", v->length_);
			put_minithumbnail(j, v->minithumbnail_.get());
			if (!v->waveform_.empty())
				j["waveform"] = base64(v->waveform_);
			put_transcript(j, v->speech_recognition_result_.get());
		}
		put_flag(j, "secret", m.is_secret_);
		break;
	}
	case td_api::messageSticker::ID: {
		auto &m = static_cast<const td_api::messageSticker &>(c);
		if (auto *st = m.sticker_.get()) {
			put_str(j, "emoji", st->emoji_);
			put_id(j, "sticker_id", st->id_);
			put_id(j, "set_id", st->set_id_);
			put_str(j, "format", sticker_format_name(st->format_.get()));
			put_num(j, "width", st->width_);
			put_num(j, "height", st->height_);
			if (st->full_type_) {
				switch (st->full_type_->get_id()) {
				case td_api::stickerFullTypeRegular::ID:
					j["sticker_type"] = "regular";
					break;
				case td_api::stickerFullTypeMask::ID:
					j["sticker_type"] = "mask";
					break;
				case td_api::stickerFullTypeCustomEmoji::ID:
					j["sticker_type"] = "custom_emoji";
					put_id(j, "custom_emoji_id",
					       static_cast<const td_api::stickerFullTypeCustomEmoji &>(
						       *st->full_type_).custom_emoji_id_);
					break;
				default:
					break;
				}
			}
		}
		put_flag(j, "premium", m.is_premium_);
		break;
	}
	case td_api::messagePaidMedia::ID: {
		auto &m = static_cast<const td_api::messagePaidMedia &>(c);
		put_num(j, "star_count", m.star_count_);
		put_flag(j, "caption_above", m.show_caption_above_media_);
		nlohmann::json items = nlohmann::json::array();
		for (const auto &pm : m.media_) {
			if (!pm)
				continue;
			nlohmann::json e = nlohmann::json::object();
			switch (pm->get_id()) {
			case td_api::paidMediaPhoto::ID: {
				auto &x = static_cast<const td_api::paidMediaPhoto &>(*pm);
				e["kind"] = "photo";
				put_photo(e, x.photo_.get());
				break;
			}
			case td_api::paidMediaVideo::ID: {
				auto &x = static_cast<const td_api::paidMediaVideo &>(*pm);
				e["kind"] = "video";
				put_video(e, x.video_.get());
				put_num(e, "start_timestamp", x.start_timestamp_);
				put_flag(e, "cover", x.cover_ != nullptr);
				break;
			}
			case td_api::paidMediaPreview::ID: {
				auto &x = static_cast<const td_api::paidMediaPreview &>(*pm);
				e["kind"] = "preview";
				put_num(e, "width", x.width_);
				put_num(e, "height", x.height_);
				put_num(e, "duration", x.duration_);
				put_minithumbnail(e, x.minithumbnail_.get());
				break;
			}
			default:
				e["kind"] = "unsupported";
				break;
			}
			items.push_back(e);
		}
		if (!items.empty())
			j["items"] = items;
		break;
	}
	default:
		is_media = false;
		return std::nullopt;
	}

	if (j.empty())
		return std::nullopt;
	return j.dump();
}


/* A TDLib message id as the server id stored in the database, or 0 for an
 * id that is not a server message's (see to_server_msg_id in TDLib.cpp). */
int64_t server_msg_id(int64_t td_id)
{
	constexpr int64_t kShift = 1048576; /* 2^20 */
	return (td_id > 0 && td_id % kShift == 0) ? td_id / kShift : 0;
}

void put_double(nlohmann::json &j, const char *key, double v)
{
	if (v != 0.0)
		j[key] = v;
}

void put_location(nlohmann::json &j, const td_api::location *l)
{
	if (!l)
		return;
	j["lat"] = l->latitude_;
	j["lon"] = l->longitude_;
	put_double(j, "accuracy", l->horizontal_accuracy_);
}

/* A formattedText as {text, entities}, entities only when there are any. */
void put_formatted(nlohmann::json &j, const char *key,
		   const td_api::formattedText *ft)
{
	if (!ft || ft->text_.empty())
		return;
	nlohmann::json o = { { "text", ft->text_ } };
	if (auto e = format_entities_json(*ft))
		o["entities"] = nlohmann::json::parse(*e);
	j[key] = o;
}

/* A message sender as {"user_id"} or {"chat_id"}. */
nlohmann::json sender_json(const td_api::MessageSender *s)
{
	if (!s)
		return nullptr;
	if (s->get_id() == td_api::messageSenderUser::ID)
		return { { "user_id",
			   static_cast<const td_api::messageSenderUser &>(*s)
				   .user_id_ } };
	if (s->get_id() == td_api::messageSenderChat::ID)
		return { { "chat_id",
			   static_cast<const td_api::messageSenderChat &>(*s)
				   .chat_id_ } };
	return nullptr;
}

const char *call_discard_reason_name(const td_api::CallDiscardReason *r)
{
	if (!r)
		return "";
	switch (r->get_id()) {
	case td_api::callDiscardReasonMissed::ID:	return "missed";
	case td_api::callDiscardReasonDeclined::ID:	return "declined";
	case td_api::callDiscardReasonDisconnected::ID:	return "disconnected";
	case td_api::callDiscardReasonHungUp::ID:	return "hung_up";
	case td_api::callDiscardReasonUpgradeToGroupCall::ID:
		return "upgrade_to_group_call";
	default:					return "";
	}
}

/*
 * The curated content_extra of the places, people, games and calls of
 * message-types.md section 4.2.2, or nullopt when @c is not one of them
 * (@handled false) or carries nothing to keep. A live location's
 * remaining time (expires_in) is left out: it changes on every update with
 * nobody moving.
 */
std::optional<std::string>
structured_extra_json(const td_api::MessageContent &c, bool &handled)
{
	nlohmann::json j = nlohmann::json::object();
	handled = true;

	switch (c.get_id()) {
	case td_api::messageLocation::ID:
		put_location(j, static_cast<const td_api::messageLocation &>(c)
				   .location_.get());
		break;
	case td_api::messageLiveLocation::ID: {
		auto &m = static_cast<const td_api::messageLiveLocation &>(c);
		if (auto *l = m.location_.get()) {
			put_location(j, l->location_.get());
			put_num(j, "live_period", l->live_period_);
			put_num(j, "heading", l->heading_);
			put_num(j, "proximity_radius", l->proximity_alert_radius_);
		}
		j["live"] = true;
		break;
	}
	case td_api::messageVenue::ID: {
		auto &m = static_cast<const td_api::messageVenue &>(c);
		if (auto *v = m.venue_.get()) {
			put_location(j, v->location_.get());
			put_str(j, "title", v->title_);
			put_str(j, "address", v->address_);
			put_str(j, "provider", v->provider_);
			put_str(j, "venue_id", v->id_);
			put_str(j, "venue_type", v->type_);
		}
		break;
	}
	case td_api::messageContact::ID: {
		auto &m = static_cast<const td_api::messageContact &>(c);
		if (auto *k = m.contact_.get()) {
			put_str(j, "phone", k->phone_number_);
			put_str(j, "first_name", k->first_name_);
			put_str(j, "last_name", k->last_name_);
			put_str(j, "vcard", k->vcard_);
			put_num(j, "user_id", k->user_id_);
		}
		break;
	}
	case td_api::messageDice::ID: {
		auto &m = static_cast<const td_api::messageDice &>(c);
		put_str(j, "emoji", m.emoji_);
		put_num(j, "value", m.value_);
		break;
	}
	case td_api::messageStakeDice::ID: {
		auto &m = static_cast<const td_api::messageStakeDice &>(c);
		put_num(j, "value", m.value_);
		put_num(j, "stake_gram_amount", m.stake_gram_amount_);
		put_num(j, "prize_gram_amount", m.prize_gram_amount_);
		j["stake"] = true;
		break;
	}
	case td_api::messageGame::ID: {
		auto &m = static_cast<const td_api::messageGame &>(c);
		if (auto *g = m.game_.get()) {
			put_id(j, "game_id", g->id_);
			put_str(j, "short_name", g->short_name_);
			put_str(j, "title", g->title_);
			put_str(j, "description", g->description_);
			put_flag(j, "has_photo", g->photo_ != nullptr);
			put_flag(j, "has_animation", g->animation_ != nullptr);
		}
		break;
	}
	case td_api::messageStory::ID: {
		auto &m = static_cast<const td_api::messageStory &>(c);
		put_num(j, "poster_chat_id", m.story_poster_chat_id_);
		put_num(j, "story_id", m.story_id_);
		put_flag(j, "via_mention", m.via_mention_);
		break;
	}
	case td_api::messageInvoice::ID: {
		auto &m = static_cast<const td_api::messageInvoice &>(c);
		if (auto *p = m.product_info_.get()) {
			put_str(j, "title", p->title_);
			put_formatted(j, "description", p->description_.get());
			put_flag(j, "has_photo", p->photo_ != nullptr);
		}
		put_str(j, "currency", m.currency_);
		put_num(j, "total_amount", m.total_amount_);
		put_str(j, "start_parameter", m.start_parameter_);
		put_flag(j, "is_test", m.is_test_);
		put_flag(j, "need_shipping_address", m.need_shipping_address_);
		put_num(j, "receipt_message_id",
			server_msg_id(m.receipt_message_id_));
		put_flag(j, "has_paid_media", m.paid_media_ != nullptr);
		break;
	}
	case td_api::messageChecklist::ID: {
		auto &m = static_cast<const td_api::messageChecklist &>(c);
		if (auto *l = m.list_.get()) {
			nlohmann::json tasks = nlohmann::json::array();
			for (const auto &t : l->tasks_) {
				if (!t)
					continue;
				nlohmann::json e = { { "id", t->id_ } };
				if (t->text_) {
					e["text"] = t->text_->text_;
					if (auto en = format_entities_json(*t->text_))
						e["entities"] = nlohmann::json::parse(*en);
				}
				nlohmann::json by = sender_json(t->completed_by_.get());
				if (!by.is_null())
					e["completed_by"] = by;
				put_num(e, "completion_date", t->completion_date_);
				tasks.push_back(e);
			}
			if (!tasks.empty())
				j["tasks"] = tasks;
			put_flag(j, "others_can_add_tasks", l->others_can_add_tasks_);
			put_flag(j, "others_can_mark_tasks_as_done",
				 l->others_can_mark_tasks_as_done_);
		}
		break;
	}
	case td_api::messagePoll::ID: {
		/* The poll itself, which changes with every vote, is in
		 * telegram_polls; the message keeps which one it shows. */
		auto &m = static_cast<const td_api::messagePoll &>(c);
		if (m.poll_)
			put_id(j, "poll_id", m.poll_->id_);
		put_flag(j, "has_media", m.media_ != nullptr);
		break;
	}
	case td_api::messageGiveaway::ID: {
		auto &m = static_cast<const td_api::messageGiveaway &>(c);
		if (auto *p = m.parameters_.get()) {
			put_num(j, "boosted_chat_id", p->boosted_chat_id_);
			if (!p->additional_chat_ids_.empty())
				j["additional_chat_ids"] = p->additional_chat_ids_;
			put_num(j, "winners_selection_date",
				p->winners_selection_date_);
			put_flag(j, "only_new_members", p->only_new_members_);
			put_flag(j, "has_public_winners", p->has_public_winners_);
			if (!p->country_codes_.empty())
				j["country_codes"] = p->country_codes_;
			put_str(j, "prize_description", p->prize_description_);
		}
		put_num(j, "winner_count", m.winner_count_);
		if (m.prize_) {
			if (m.prize_->get_id() == td_api::giveawayPrizePremium::ID)
				j["prize"] = { { "premium_months",
					static_cast<const td_api::giveawayPrizePremium &>(
						*m.prize_).month_count_ } };
			else if (m.prize_->get_id() == td_api::giveawayPrizeStars::ID)
				j["prize"] = { { "stars",
					static_cast<const td_api::giveawayPrizeStars &>(
						*m.prize_).star_count_ } };
		}
		break;
	}
	case td_api::messageCall::ID: {
		auto &m = static_cast<const td_api::messageCall &>(c);
		put_id(j, "call_id", m.unique_id_);
		put_flag(j, "video", m.is_video_);
		put_num(j, "duration", m.duration_);
		put_str(j, "discard_reason",
			call_discard_reason_name(m.discard_reason_.get()));
		break;
	}
	case td_api::messageGroupCall::ID: {
		auto &m = static_cast<const td_api::messageGroupCall &>(c);
		put_id(j, "call_id", m.unique_id_);
		put_flag(j, "group", true);
		put_flag(j, "active", m.is_active_);
		put_flag(j, "missed", m.was_missed_);
		put_flag(j, "video", m.is_video_);
		put_num(j, "duration", m.duration_);
		nlohmann::json ps = nlohmann::json::array();
		for (const auto &s : m.other_participant_ids_) {
			nlohmann::json e = sender_json(s.get());
			if (!e.is_null())
				ps.push_back(e);
		}
		if (!ps.empty())
			j["participants"] = ps;
		break;
	}
	default:
		handled = false;
		return std::nullopt;
	}

	if (j.empty())
		return std::nullopt;
	return j.dump();
}


/*
 * "upgradedGiftOriginResale" with @prefix "upgradedGiftOrigin" ->
 * "resale": the variant of a TDLib union type, snake case, from the class
 * name TDLib's own to_string() starts with.
 */
std::string tl_variant(const td_api::Object *o, const char *prefix)
{
	if (!o)
		return std::string();
	std::string name = td_api::to_string(*o);
	name = name.substr(0, name.find_first_of(" \n{"));
	std::string p = prefix;
	if (name.rfind(p, 0) == 0)
		name = name.substr(p.size());
	std::string out;
	for (char ch : name) {
		if (ch >= 'A' && ch <= 'Z') {
			if (!out.empty())
				out += '_';
			out += (char)(ch - 'A' + 'a');
		} else {
			out += ch;
		}
	}
	return out;
}

/* A price in Telegram Stars or in grams: {"stars": n} or {"gram_cents": n}. */
nlohmann::json price_json(const td_api::Object *p)
{
	if (!p)
		return nullptr;
	switch (p->get_id()) {
	case td_api::giftResalePriceStar::ID:
		return { { "stars", static_cast<const td_api::giftResalePriceStar &>(*p).star_count_ } };
	case td_api::giftResalePriceGram::ID:
		return { { "gram_cents", static_cast<const td_api::giftResalePriceGram &>(*p).gram_cent_count_ } };
	case td_api::suggestedPostPriceStar::ID:
		return { { "stars", static_cast<const td_api::suggestedPostPriceStar &>(*p).star_count_ } };
	case td_api::suggestedPostPriceGram::ID:
		return { { "gram_cents", static_cast<const td_api::suggestedPostPriceGram &>(*p).gram_cent_count_ } };
	default:
		return nullptr;
	}
}

void put_json(nlohmann::json &j, const char *key, nlohmann::json v)
{
	if (!v.is_null() && !(v.is_array() && v.empty()))
		j[key] = std::move(v);
}

void put_msg_id(nlohmann::json &j, const char *key, int64_t td_id)
{
	put_num(j, key, server_msg_id(td_id));
}

/*
 * A background's fill: {type, colors, rotation_angle}, the type "solid",
 * "gradient" or "freeform_gradient", the colours as RGB integers
 * (0xRRGGBB) in TDLib's order (a gradient's top then bottom colour), the
 * rotation only for a gradient (absent when 0).
 */
nlohmann::json background_fill_json(const td_api::BackgroundFill *f)
{
	if (!f)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	put_str(j, "type", tl_variant(f, "backgroundFill"));
	nlohmann::json colors = nlohmann::json::array();
	switch (f->get_id()) {
	case td_api::backgroundFillSolid::ID:
		colors.push_back(static_cast<const td_api::backgroundFillSolid &>(*f).color_);
		break;
	case td_api::backgroundFillGradient::ID: {
		auto &g = static_cast<const td_api::backgroundFillGradient &>(*f);
		colors.push_back(g.top_color_);
		colors.push_back(g.bottom_color_);
		put_num(j, "rotation_angle", g.rotation_angle_);
		break;
	}
	case td_api::backgroundFillFreeformGradient::ID:
		for (int32_t c : static_cast<const td_api::backgroundFillFreeformGradient &>(*f).colors_)
			colors.push_back(c);
		break;
	default:
		break;
	}
	j["colors"] = colors;
	return j;
}

/*
 * The fields of a background's type, beside its "type" key: a wallpaper's
 * is_blurred and is_moving; a pattern's fill, intensity, is_inverted and
 * is_moving; a fill's fill; a chat theme's theme_name.
 */
void put_background_type(nlohmann::json &bg, const td_api::BackgroundType *t)
{
	if (!t)
		return;
	switch (t->get_id()) {
	case td_api::backgroundTypeWallpaper::ID: {
		auto &w = static_cast<const td_api::backgroundTypeWallpaper &>(*t);
		put_flag(bg, "is_blurred", w.is_blurred_);
		put_flag(bg, "is_moving", w.is_moving_);
		break;
	}
	case td_api::backgroundTypePattern::ID: {
		auto &p = static_cast<const td_api::backgroundTypePattern &>(*t);
		put_json(bg, "fill", background_fill_json(p.fill_.get()));
		put_num(bg, "intensity", p.intensity_);
		put_flag(bg, "is_inverted", p.is_inverted_);
		put_flag(bg, "is_moving", p.is_moving_);
		break;
	}
	case td_api::backgroundTypeFill::ID:
		put_json(bg, "fill", background_fill_json(
			static_cast<const td_api::backgroundTypeFill &>(*t).fill_.get()));
		break;
	case td_api::backgroundTypeChatTheme::ID:
		put_str(bg, "theme_name",
			static_cast<const td_api::backgroundTypeChatTheme &>(*t).theme_name_);
		break;
	default:
		break;
	}
}

nlohmann::json chat_photo_json(const td_api::chatPhoto *p)
{
	if (!p)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	put_id(j, "photo_id", p->id_);
	const td_api::photoSize *best = nullptr;
	for (const auto &sz : p->sizes_)
		if (sz && (!best || sz->width_ > best->width_))
			best = sz.get();
	if (best) {
		put_num(j, "width", best->width_);
		put_num(j, "height", best->height_);
	}
	put_flag(j, "animated", p->animation_ != nullptr);
	put_minithumbnail(j, p->minithumbnail_.get());
	return j;
}

nlohmann::json gift_json(const td_api::gift *g)
{
	if (!g)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	put_id(j, "id", g->id_);
	put_num(j, "star_count", g->star_count_);
	put_num(j, "upgrade_star_count", g->upgrade_star_count_);
	put_num(j, "publisher_chat_id", g->publisher_chat_id_);
	put_flag(j, "for_birthday", g->is_for_birthday_);
	put_flag(j, "premium", g->is_premium_);
	if (g->sticker_)
		put_str(j, "emoji", g->sticker_->emoji_);
	return j;
}

/*
 * The rarity of an upgraded gift's model, symbol or backdrop into @j:
 * "rarity_per_mille" (kept at 0, which means under 0.1%) or, for a named
 * rarity, "rarity": "uncommon", "rare", "epic" or "legendary".
 */
void put_gift_rarity(nlohmann::json &j,
		     const td_api::UpgradedGiftAttributeRarity *r)
{
	if (!r)
		return;
	if (r->get_id() == td_api::upgradedGiftAttributeRarityPerMille::ID)
		j["rarity_per_mille"] = static_cast<
			const td_api::upgradedGiftAttributeRarityPerMille &>(*r)
						.per_mille_;
	else
		put_str(j, "rarity",
			tl_variant(r, "upgradedGiftAttributeRarity"));
}

/*
 * An upgraded gift: its ids, title, name, number and owner, and what it
 * looks like: the model {name, rarity, crafted}, the symbol {name,
 * rarity}, the backdrop {name, four 0xRRGGBB colours, rarity} and the
 * details of the gift it was upgraded from. The model's and the
 * symbol's stickers are message files (role "gift"), not in the JSON.
 */
nlohmann::json upgraded_gift_json(const td_api::upgradedGift *g)
{
	if (!g)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	put_id(j, "id", g->id_);
	put_id(j, "regular_gift_id", g->regular_gift_id_);
	put_str(j, "title", g->title_);
	put_str(j, "name", g->name_);
	put_num(j, "number", g->number_);
	put_num(j, "publisher_chat_id", g->publisher_chat_id_);
	put_json(j, "owner", sender_json(g->owner_id_.get()));
	if (auto *m = g->model_.get()) {
		nlohmann::json o = nlohmann::json::object();
		put_str(o, "name", m->name_);
		put_gift_rarity(o, m->rarity_.get());
		put_flag(o, "crafted", m->is_crafted_);
		j["model"] = std::move(o);
	}
	if (auto *s = g->symbol_.get()) {
		nlohmann::json o = nlohmann::json::object();
		put_str(o, "name", s->name_);
		put_gift_rarity(o, s->rarity_.get());
		j["symbol"] = std::move(o);
	}
	if (auto *b = g->backdrop_.get()) {
		/* A colour keeps its value even when 0 (black). */
		nlohmann::json o = nlohmann::json::object();
		put_str(o, "name", b->name_);
		if (auto *c = b->colors_.get()) {
			o["center_color"] = c->center_color_;
			o["edge_color"] = c->edge_color_;
			o["symbol_color"] = c->symbol_color_;
			o["text_color"] = c->text_color_;
		}
		put_gift_rarity(o, b->rarity_.get());
		j["backdrop"] = std::move(o);
	}
	if (auto *d = g->original_details_.get()) {
		nlohmann::json o = nlohmann::json::object();
		put_json(o, "sender", sender_json(d->sender_id_.get()));
		put_json(o, "receiver", sender_json(d->receiver_id_.get()));
		put_formatted(o, "text", d->text_.get());
		put_num(o, "date", d->date_);
		j["original_details"] = std::move(o);
	}
	return j;
}

/*
 * The curated content_extra of a service message (message-types.md section
 * 4.2.3): the action's own fields. Message ids are server ids. Left out on
 * purpose: a Premium gift code (whoever reads the archive could redeem it),
 * an encrypted TON comment, a payment's bot-side payload and order info,
 * encrypted passport data, and flags that only describe what this account
 * may do (can_*).
 */
std::optional<std::string> service_extra_json(const td_api::MessageContent &c)
{
	nlohmann::json j = nlohmann::json::object();

	switch (c.get_id()) {
	case td_api::messageBasicGroupChatCreate::ID: {
		auto &m = static_cast<const td_api::messageBasicGroupChatCreate &>(c);
		put_str(j, "title", m.title_);
		put_json(j, "user_ids", m.member_user_ids_);
		break;
	}
	case td_api::messageSupergroupChatCreate::ID:
		put_str(j, "title", static_cast<const td_api::messageSupergroupChatCreate &>(c).title_);
		break;
	case td_api::messageChatChangeTitle::ID:
		put_str(j, "title", static_cast<const td_api::messageChatChangeTitle &>(c).title_);
		break;
	case td_api::messageChatChangePhoto::ID:
		put_json(j, "photo", chat_photo_json(
			static_cast<const td_api::messageChatChangePhoto &>(c).photo_.get()));
		break;
	case td_api::messageChatAddMembers::ID:
		put_json(j, "user_ids", static_cast<const td_api::messageChatAddMembers &>(c).member_user_ids_);
		break;
	case td_api::messageChatJoinFromCommunity::ID:
		put_num(j, "community_id", static_cast<const td_api::messageChatJoinFromCommunity &>(c).community_id_);
		break;
	case td_api::messageChatDeleteMember::ID:
		put_num(j, "user_id", static_cast<const td_api::messageChatDeleteMember &>(c).user_id_);
		break;
	case td_api::messageChatAddedToCommunity::ID:
		put_num(j, "community_id", static_cast<const td_api::messageChatAddedToCommunity &>(c).community_id_);
		break;
	case td_api::messageChatUpgradeTo::ID:
		put_num(j, "supergroup_id", static_cast<const td_api::messageChatUpgradeTo &>(c).supergroup_id_);
		break;
	case td_api::messageChatUpgradeFrom::ID: {
		auto &m = static_cast<const td_api::messageChatUpgradeFrom &>(c);
		put_str(j, "title", m.title_);
		put_num(j, "basic_group_id", m.basic_group_id_);
		break;
	}
	case td_api::messageChatOwnerLeft::ID:
		put_num(j, "new_owner_user_id", static_cast<const td_api::messageChatOwnerLeft &>(c).new_owner_user_id_);
		break;
	case td_api::messageChatOwnerChanged::ID:
		put_num(j, "new_owner_user_id", static_cast<const td_api::messageChatOwnerChanged &>(c).new_owner_user_id_);
		break;
	case td_api::messageChatHasProtectedContentToggled::ID: {
		auto &m = static_cast<const td_api::messageChatHasProtectedContentToggled &>(c);
		j["old_value"] = m.old_has_protected_content_;
		j["new_value"] = m.new_has_protected_content_;
		put_msg_id(j, "request_message_id", m.request_message_id_);
		break;
	}
	case td_api::messageChatHasProtectedContentDisableRequested::ID:
		put_flag(j, "expired", static_cast<const td_api::messageChatHasProtectedContentDisableRequested &>(c).is_expired_);
		break;
	case td_api::messagePinMessage::ID:
		put_msg_id(j, "message_id", static_cast<const td_api::messagePinMessage &>(c).message_id_);
		break;
	case td_api::messageChatSetBackground::ID: {
		auto &m = static_cast<const td_api::messageChatSetBackground &>(c);
		put_msg_id(j, "old_background_message_id", m.old_background_message_id_);
		put_flag(j, "only_for_self", m.only_for_self_);
		if (m.background_ && m.background_->background_) {
			auto &b = *m.background_->background_;
			nlohmann::json bg = nlohmann::json::object();
			put_id(bg, "id", b.id_);
			put_str(bg, "name", b.name_);
			put_flag(bg, "dark", b.is_dark_);
			put_str(bg, "type", tl_variant(b.type_.get(), "backgroundType"));
			put_num(bg, "dark_theme_dimming", m.background_->dark_theme_dimming_);
			put_background_type(bg, b.type_.get());
			j["background"] = bg;
		}
		break;
	}
	case td_api::messageChatSetTheme::ID: {
		auto &m = static_cast<const td_api::messageChatSetTheme &>(c);
		if (m.theme_ && m.theme_->get_id() == td_api::chatThemeEmoji::ID)
			put_str(j, "theme_name", static_cast<const td_api::chatThemeEmoji &>(*m.theme_).name_);
		else if (m.theme_)
			put_str(j, "theme_kind", tl_variant(m.theme_.get(), "chatTheme"));
		/* A gift theme names its upgraded gift. */
		if (m.theme_ && m.theme_->get_id() == td_api::chatThemeGift::ID) {
			auto &t = static_cast<const td_api::chatThemeGift &>(*m.theme_);
			if (t.gift_theme_ && t.gift_theme_->gift_) {
				put_id(j, "gift_id", t.gift_theme_->gift_->id_);
				put_str(j, "gift_name", t.gift_theme_->gift_->name_);
			}
		}
		break;
	}
	case td_api::messageChatSetMessageAutoDeleteTime::ID: {
		auto &m = static_cast<const td_api::messageChatSetMessageAutoDeleteTime &>(c);
		j["seconds"] = m.message_auto_delete_time_;
		put_num(j, "from_user_id", m.from_user_id_);
		break;
	}
	case td_api::messageChatBoost::ID:
		put_num(j, "boost_count", static_cast<const td_api::messageChatBoost &>(c).boost_count_);
		break;
	case td_api::messageForumTopicCreated::ID: {
		auto &m = static_cast<const td_api::messageForumTopicCreated &>(c);
		put_str(j, "name", m.name_);
		put_flag(j, "name_implicit", m.is_name_implicit_);
		if (m.icon_) {
			put_num(j, "icon_color", m.icon_->color_);
			put_id(j, "icon_custom_emoji_id", m.icon_->custom_emoji_id_);
		}
		break;
	}
	case td_api::messageForumTopicEdited::ID: {
		auto &m = static_cast<const td_api::messageForumTopicEdited &>(c);
		put_str(j, "name", m.name_);
		if (m.edit_icon_custom_emoji_id_) {
			j["icon_edited"] = true;
			put_id(j, "icon_custom_emoji_id", m.icon_custom_emoji_id_);
		}
		break;
	}
	case td_api::messageForumTopicIsClosedToggled::ID:
		j["closed"] = static_cast<const td_api::messageForumTopicIsClosedToggled &>(c).is_closed_;
		break;
	case td_api::messageForumTopicIsHiddenToggled::ID:
		j["hidden"] = static_cast<const td_api::messageForumTopicIsHiddenToggled &>(c).is_hidden_;
		break;
	case td_api::messageSuggestProfilePhoto::ID:
		put_json(j, "photo", chat_photo_json(
			static_cast<const td_api::messageSuggestProfilePhoto &>(c).photo_.get()));
		break;
	case td_api::messageSuggestBirthdate::ID:
		if (auto *b = static_cast<const td_api::messageSuggestBirthdate &>(c).birthdate_.get()) {
			put_num(j, "day", b->day_);
			put_num(j, "month", b->month_);
			put_num(j, "year", b->year_);
		}
		break;
	case td_api::messageGameScore::ID: {
		auto &m = static_cast<const td_api::messageGameScore &>(c);
		put_msg_id(j, "game_message_id", m.game_message_id_);
		put_id(j, "game_id", m.game_id_);
		j["score"] = m.score_;
		break;
	}
	case td_api::messageManagedBotCreated::ID: {
		auto &m = static_cast<const td_api::messageManagedBotCreated &>(c);
		put_num(j, "bot_user_id", m.bot_user_id_);
		put_num(j, "manager_bot_user_id", m.manager_bot_user_id_);
		break;
	}
	case td_api::messageVideoChatScheduled::ID: {
		auto &m = static_cast<const td_api::messageVideoChatScheduled &>(c);
		put_num(j, "group_call_id", m.group_call_id_);
		put_num(j, "start_date", m.start_date_);
		break;
	}
	case td_api::messageVideoChatStarted::ID:
		put_num(j, "group_call_id", static_cast<const td_api::messageVideoChatStarted &>(c).group_call_id_);
		break;
	case td_api::messageVideoChatEnded::ID:
		put_num(j, "duration", static_cast<const td_api::messageVideoChatEnded &>(c).duration_);
		break;
	case td_api::messageInviteVideoChatParticipants::ID: {
		auto &m = static_cast<const td_api::messageInviteVideoChatParticipants &>(c);
		put_num(j, "group_call_id", m.group_call_id_);
		put_json(j, "user_ids", m.user_ids_);
		break;
	}
	case td_api::messagePollOptionAdded::ID: {
		auto &m = static_cast<const td_api::messagePollOptionAdded &>(c);
		put_msg_id(j, "poll_message_id", m.poll_message_id_);
		put_str(j, "option_id", m.option_id_);
		put_formatted(j, "option", m.text_.get());
		break;
	}
	case td_api::messagePollOptionDeleted::ID: {
		auto &m = static_cast<const td_api::messagePollOptionDeleted &>(c);
		put_msg_id(j, "poll_message_id", m.poll_message_id_);
		put_str(j, "option_id", m.option_id_);
		put_formatted(j, "option", m.text_.get());
		break;
	}
	case td_api::messageChecklistTasksDone::ID: {
		auto &m = static_cast<const td_api::messageChecklistTasksDone &>(c);
		put_msg_id(j, "checklist_message_id", m.checklist_message_id_);
		put_json(j, "done_task_ids", m.marked_as_done_task_ids_);
		put_json(j, "not_done_task_ids", m.marked_as_not_done_task_ids_);
		break;
	}
	case td_api::messageChecklistTasksAdded::ID: {
		auto &m = static_cast<const td_api::messageChecklistTasksAdded &>(c);
		put_msg_id(j, "checklist_message_id", m.checklist_message_id_);
		nlohmann::json tasks = nlohmann::json::array();
		for (const auto &t : m.tasks_) {
			if (!t)
				continue;
			nlohmann::json e = { { "id", t->id_ } };
			if (t->text_) {
				e["text"] = t->text_->text_;
				if (auto en = format_entities_json(*t->text_))
					e["entities"] = nlohmann::json::parse(*en);
			}
			tasks.push_back(e);
		}
		put_json(j, "tasks", tasks);
		break;
	}
	case td_api::messagePaymentSuccessful::ID: {
		auto &m = static_cast<const td_api::messagePaymentSuccessful &>(c);
		put_num(j, "invoice_chat_id", m.invoice_chat_id_);
		put_msg_id(j, "invoice_message_id", m.invoice_message_id_);
		put_str(j, "currency", m.currency_);
		put_num(j, "total_amount", m.total_amount_);
		put_num(j, "subscription_until_date", m.subscription_until_date_);
		put_flag(j, "recurring", m.is_recurring_);
		put_flag(j, "first_recurring", m.is_first_recurring_);
		put_str(j, "invoice_name", m.invoice_name_);
		break;
	}
	case td_api::messagePaymentSuccessfulBot::ID: {
		auto &m = static_cast<const td_api::messagePaymentSuccessfulBot &>(c);
		put_str(j, "currency", m.currency_);
		put_num(j, "total_amount", m.total_amount_);
		put_num(j, "subscription_until_date", m.subscription_until_date_);
		put_flag(j, "recurring", m.is_recurring_);
		put_flag(j, "first_recurring", m.is_first_recurring_);
		put_str(j, "telegram_payment_charge_id", m.telegram_payment_charge_id_);
		put_str(j, "provider_payment_charge_id", m.provider_payment_charge_id_);
		break;
	}
	case td_api::messagePaymentRefunded::ID: {
		auto &m = static_cast<const td_api::messagePaymentRefunded &>(c);
		put_json(j, "owner", sender_json(m.owner_id_.get()));
		put_str(j, "currency", m.currency_);
		put_num(j, "total_amount", m.total_amount_);
		put_str(j, "telegram_payment_charge_id", m.telegram_payment_charge_id_);
		put_str(j, "provider_payment_charge_id", m.provider_payment_charge_id_);
		break;
	}
	case td_api::messageGiftedPremium::ID: {
		auto &m = static_cast<const td_api::messageGiftedPremium &>(c);
		put_num(j, "gifter_user_id", m.gifter_user_id_);
		put_num(j, "receiver_user_id", m.receiver_user_id_);
		put_formatted(j, "text", m.text_.get());
		put_str(j, "currency", m.currency_);
		put_num(j, "amount", m.amount_);
		put_str(j, "cryptocurrency", m.cryptocurrency_);
		put_id(j, "cryptocurrency_amount", m.cryptocurrency_amount_);
		put_num(j, "month_count", m.month_count_);
		put_num(j, "day_count", m.day_count_);
		break;
	}
	case td_api::messagePremiumGiftCode::ID: {
		auto &m = static_cast<const td_api::messagePremiumGiftCode &>(c);
		put_json(j, "creator", sender_json(m.creator_id_.get()));
		put_formatted(j, "text", m.text_.get());
		put_flag(j, "from_giveaway", m.is_from_giveaway_);
		put_flag(j, "unclaimed", m.is_unclaimed_);
		put_str(j, "currency", m.currency_);
		put_num(j, "amount", m.amount_);
		put_str(j, "cryptocurrency", m.cryptocurrency_);
		put_id(j, "cryptocurrency_amount", m.cryptocurrency_amount_);
		put_num(j, "month_count", m.month_count_);
		put_num(j, "day_count", m.day_count_);
		put_flag(j, "has_code", !m.code_.empty());
		break;
	}
	case td_api::messageGiveawayCreated::ID:
		put_num(j, "star_count", static_cast<const td_api::messageGiveawayCreated &>(c).star_count_);
		break;
	case td_api::messageGiveawayCompleted::ID: {
		auto &m = static_cast<const td_api::messageGiveawayCompleted &>(c);
		put_msg_id(j, "giveaway_message_id", m.giveaway_message_id_);
		put_num(j, "winner_count", m.winner_count_);
		put_flag(j, "star_giveaway", m.is_star_giveaway_);
		put_num(j, "unclaimed_prize_count", m.unclaimed_prize_count_);
		break;
	}
	case td_api::messageGiveawayWinners::ID: {
		auto &m = static_cast<const td_api::messageGiveawayWinners &>(c);
		put_num(j, "boosted_chat_id", m.boosted_chat_id_);
		put_msg_id(j, "giveaway_message_id", m.giveaway_message_id_);
		put_num(j, "additional_chat_count", m.additional_chat_count_);
		put_num(j, "winners_selection_date", m.actual_winners_selection_date_);
		put_flag(j, "only_new_members", m.only_new_members_);
		put_flag(j, "refunded", m.was_refunded_);
		if (m.prize_) {
			if (m.prize_->get_id() == td_api::giveawayPrizePremium::ID)
				j["prize"] = { { "premium_months", static_cast<const td_api::giveawayPrizePremium &>(*m.prize_).month_count_ } };
			else if (m.prize_->get_id() == td_api::giveawayPrizeStars::ID)
				j["prize"] = { { "stars", static_cast<const td_api::giveawayPrizeStars &>(*m.prize_).star_count_ } };
		}
		put_str(j, "prize_description", m.prize_description_);
		put_num(j, "winner_count", m.winner_count_);
		put_json(j, "winner_user_ids", m.winner_user_ids_);
		put_num(j, "unclaimed_prize_count", m.unclaimed_prize_count_);
		break;
	}
	case td_api::messageGiftedStars::ID: {
		auto &m = static_cast<const td_api::messageGiftedStars &>(c);
		put_num(j, "gifter_user_id", m.gifter_user_id_);
		put_num(j, "receiver_user_id", m.receiver_user_id_);
		put_str(j, "currency", m.currency_);
		put_num(j, "amount", m.amount_);
		put_str(j, "cryptocurrency", m.cryptocurrency_);
		put_id(j, "cryptocurrency_amount", m.cryptocurrency_amount_);
		put_num(j, "star_count", m.star_count_);
		put_str(j, "transaction_id", m.transaction_id_);
		break;
	}
	case td_api::messageGiftedGrams::ID: {
		auto &m = static_cast<const td_api::messageGiftedGrams &>(c);
		put_num(j, "gifter_user_id", m.gifter_user_id_);
		put_num(j, "receiver_user_id", m.receiver_user_id_);
		put_num(j, "gram_amount", m.gram_amount_);
		put_str(j, "transaction_id", m.transaction_id_);
		break;
	}
	case td_api::messageGiveawayPrizeStars::ID: {
		auto &m = static_cast<const td_api::messageGiveawayPrizeStars &>(c);
		put_num(j, "star_count", m.star_count_);
		put_str(j, "transaction_id", m.transaction_id_);
		put_num(j, "boosted_chat_id", m.boosted_chat_id_);
		put_msg_id(j, "giveaway_message_id", m.giveaway_message_id_);
		put_flag(j, "unclaimed", m.is_unclaimed_);
		break;
	}
	case td_api::messageTonWalletTransfer::ID: {
		auto &m = static_cast<const td_api::messageTonWalletTransfer &>(c);
		put_num(j, "sender_user_id", m.sender_user_id_);
		put_str(j, "transaction_id", m.transaction_id_);
		put_str(j, "peer_address", m.peer_address_);
		put_id(j, "amount", m.amount_);
		if (m.is_comment_encrypted_)
			put_flag(j, "comment_encrypted", true);
		else
			put_str(j, "comment", m.comment_);
		break;
	}
	case td_api::messageTonConnectRequest::ID: {
		auto &m = static_cast<const td_api::messageTonConnectRequest &>(c);
		put_id(j, "session_id", m.session_id_);
		put_str(j, "state", tl_variant(m.state_.get(), "tonConnectRequestState"));
		put_str(j, "dapp_name", m.dapp_name_);
		put_str(j, "topic", m.topic_);
		break;
	}
	case td_api::messageGift::ID: {
		auto &m = static_cast<const td_api::messageGift &>(c);
		put_json(j, "gift", gift_json(m.gift_.get()));
		put_json(j, "sender", sender_json(m.sender_id_.get()));
		put_json(j, "receiver", sender_json(m.receiver_id_.get()));
		put_str(j, "received_gift_id", m.received_gift_id_);
		put_formatted(j, "text", m.text_.get());
		put_num(j, "unique_gift_number", m.unique_gift_number_);
		put_num(j, "sell_star_count", m.sell_star_count_);
		put_num(j, "prepaid_upgrade_star_count", m.prepaid_upgrade_star_count_);
		put_flag(j, "upgrade_separate", m.is_upgrade_separate_);
		put_flag(j, "from_auction", m.is_from_auction_);
		put_flag(j, "private", m.is_private_);
		put_flag(j, "saved", m.is_saved_);
		put_flag(j, "prepaid_upgrade", m.is_prepaid_upgrade_);
		put_flag(j, "converted", m.was_converted_);
		put_flag(j, "upgraded", m.was_upgraded_);
		put_flag(j, "refunded", m.was_refunded_);
		put_str(j, "upgraded_received_gift_id", m.upgraded_received_gift_id_);
		break;
	}
	case td_api::messageUpgradedGift::ID: {
		auto &m = static_cast<const td_api::messageUpgradedGift &>(c);
		put_json(j, "gift", upgraded_gift_json(m.gift_.get()));
		put_json(j, "sender", sender_json(m.sender_id_.get()));
		put_json(j, "receiver", sender_json(m.receiver_id_.get()));
		put_str(j, "origin", tl_variant(m.origin_.get(), "upgradedGiftOrigin"));
		if (m.origin_ && m.origin_->get_id() == td_api::upgradedGiftOriginUpgrade::ID)
			put_msg_id(j, "gift_message_id", static_cast<const td_api::upgradedGiftOriginUpgrade &>(*m.origin_).gift_message_id_);
		put_str(j, "received_gift_id", m.received_gift_id_);
		put_formatted(j, "text", m.text_.get());
		put_flag(j, "private", m.is_private_);
		put_flag(j, "saved", m.is_saved_);
		put_flag(j, "transferred", m.was_transferred_);
		put_num(j, "transfer_star_count", m.transfer_star_count_);
		put_num(j, "next_transfer_date", m.next_transfer_date_);
		put_num(j, "next_resale_date", m.next_resale_date_);
		put_num(j, "export_date", m.export_date_);
		put_num(j, "craft_date", m.craft_date_);
		break;
	}
	case td_api::messageRefundedUpgradedGift::ID: {
		auto &m = static_cast<const td_api::messageRefundedUpgradedGift &>(c);
		put_json(j, "gift", gift_json(m.gift_.get()));
		put_json(j, "sender", sender_json(m.sender_id_.get()));
		put_json(j, "receiver", sender_json(m.receiver_id_.get()));
		put_str(j, "origin", tl_variant(m.origin_.get(), "upgradedGiftOrigin"));
		break;
	}
	case td_api::messageUpgradedGiftPurchaseOffer::ID: {
		auto &m = static_cast<const td_api::messageUpgradedGiftPurchaseOffer &>(c);
		put_json(j, "gift", upgraded_gift_json(m.gift_.get()));
		put_str(j, "state", tl_variant(m.state_.get(), "giftPurchaseOfferState"));
		put_json(j, "price", price_json(m.price_.get()));
		put_num(j, "expiration_date", m.expiration_date_);
		break;
	}
	case td_api::messageUpgradedGiftPurchaseOfferRejected::ID: {
		auto &m = static_cast<const td_api::messageUpgradedGiftPurchaseOfferRejected &>(c);
		put_json(j, "gift", upgraded_gift_json(m.gift_.get()));
		put_json(j, "price", price_json(m.price_.get()));
		put_msg_id(j, "offer_message_id", m.offer_message_id_);
		put_flag(j, "expired", m.was_expired_);
		break;
	}
	case td_api::messagePaidMessagesRefunded::ID: {
		auto &m = static_cast<const td_api::messagePaidMessagesRefunded &>(c);
		put_num(j, "message_count", m.message_count_);
		put_num(j, "star_count", m.star_count_);
		break;
	}
	case td_api::messagePaidMessagePriceChanged::ID:
		j["paid_message_star_count"] = static_cast<const td_api::messagePaidMessagePriceChanged &>(c).paid_message_star_count_;
		break;
	case td_api::messageDirectMessagePriceChanged::ID: {
		auto &m = static_cast<const td_api::messageDirectMessagePriceChanged &>(c);
		j["enabled"] = m.is_enabled_;
		j["paid_message_star_count"] = m.paid_message_star_count_;
		break;
	}
	case td_api::messageSuggestedPostApprovalFailed::ID: {
		auto &m = static_cast<const td_api::messageSuggestedPostApprovalFailed &>(c);
		put_msg_id(j, "suggested_post_message_id", m.suggested_post_message_id_);
		put_json(j, "price", price_json(m.price_.get()));
		break;
	}
	case td_api::messageSuggestedPostApproved::ID: {
		auto &m = static_cast<const td_api::messageSuggestedPostApproved &>(c);
		put_msg_id(j, "suggested_post_message_id", m.suggested_post_message_id_);
		put_json(j, "price", price_json(m.price_.get()));
		put_num(j, "send_date", m.send_date_);
		break;
	}
	case td_api::messageSuggestedPostDeclined::ID: {
		auto &m = static_cast<const td_api::messageSuggestedPostDeclined &>(c);
		put_msg_id(j, "suggested_post_message_id", m.suggested_post_message_id_);
		put_str(j, "comment", m.comment_);
		break;
	}
	case td_api::messageSuggestedPostPaid::ID: {
		auto &m = static_cast<const td_api::messageSuggestedPostPaid &>(c);
		put_msg_id(j, "suggested_post_message_id", m.suggested_post_message_id_);
		if (m.star_amount_) {
			put_num(j, "star_count", m.star_amount_->star_count_);
			put_num(j, "nanostar_count", m.star_amount_->nanostar_count_);
		}
		put_num(j, "gram_amount", m.gram_amount_);
		break;
	}
	case td_api::messageSuggestedPostRefunded::ID: {
		auto &m = static_cast<const td_api::messageSuggestedPostRefunded &>(c);
		put_msg_id(j, "suggested_post_message_id", m.suggested_post_message_id_);
		put_str(j, "reason", tl_variant(m.reason_.get(), "suggestedPostRefundReason"));
		break;
	}
	case td_api::messageUsersShared::ID: {
		auto &m = static_cast<const td_api::messageUsersShared &>(c);
		nlohmann::json users = nlohmann::json::array();
		for (const auto &u : m.users_) {
			if (!u)
				continue;
			nlohmann::json e = { { "user_id", u->user_id_ } };
			put_str(e, "first_name", u->first_name_);
			put_str(e, "last_name", u->last_name_);
			put_str(e, "username", u->username_);
			users.push_back(e);
		}
		put_json(j, "users", users);
		put_num(j, "button_id", m.button_id_);
		break;
	}
	case td_api::messageChatShared::ID: {
		auto &m = static_cast<const td_api::messageChatShared &>(c);
		if (m.chat_) {
			nlohmann::json e = { { "chat_id", m.chat_->chat_id_ } };
			put_str(e, "title", m.chat_->title_);
			put_str(e, "username", m.chat_->username_);
			j["chat"] = e;
		}
		put_num(j, "button_id", m.button_id_);
		break;
	}
	case td_api::messageBotWriteAccessAllowed::ID: {
		auto &m = static_cast<const td_api::messageBotWriteAccessAllowed &>(c);
		put_str(j, "reason", tl_variant(m.reason_.get(), "botWriteAccessAllowReason"));
		if (m.reason_ && m.reason_->get_id() == td_api::botWriteAccessAllowReasonConnectedWebsite::ID)
			put_str(j, "domain_name", static_cast<const td_api::botWriteAccessAllowReasonConnectedWebsite &>(*m.reason_).domain_name_);
		break;
	}
	case td_api::messageWebAppDataSent::ID:
		put_str(j, "button_text", static_cast<const td_api::messageWebAppDataSent &>(c).button_text_);
		break;
	case td_api::messageWebAppDataReceived::ID: {
		auto &m = static_cast<const td_api::messageWebAppDataReceived &>(c);
		put_str(j, "button_text", m.button_text_);
		put_str(j, "data", m.data_);
		break;
	}
	case td_api::messagePassportDataSent::ID: {
		nlohmann::json types = nlohmann::json::array();
		for (const auto &t : static_cast<const td_api::messagePassportDataSent &>(c).types_)
			types.push_back(tl_variant(t.get(), "passportElementType"));
		put_json(j, "types", types);
		break;
	}
	case td_api::messagePassportDataReceived::ID: {
		nlohmann::json types = nlohmann::json::array();
		for (const auto &e : static_cast<const td_api::messagePassportDataReceived &>(c).elements_)
			if (e)
				types.push_back(tl_variant(e->type_.get(), "passportElementType"));
		put_json(j, "types", types);
		break;
	}
	case td_api::messageProximityAlertTriggered::ID: {
		auto &m = static_cast<const td_api::messageProximityAlertTriggered &>(c);
		put_json(j, "traveler", sender_json(m.traveler_id_.get()));
		put_json(j, "watcher", sender_json(m.watcher_id_.get()));
		put_num(j, "distance", m.distance_);
		break;
	}
	default:
		/* Nothing beyond the class and the description: a join by
		 * link, a screenshot, a custom action whose text is the
		 * description, ... */
		break;
	}

	if (j.empty())
		return std::nullopt;
	return j.dump();
}

/*
 * A poll's, an explanation's or an option's media as a curated object: its
 * kind and what a reader needs of it. Its file, if it has one, is in
 * telegram_message_files (role "poll").
 */
nlohmann::json poll_media_json(const td_api::PollMedia *pm)
{
	if (!pm)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	switch (pm->get_id()) {
	case td_api::pollMediaPhoto::ID: {
		auto &m = static_cast<const td_api::pollMediaPhoto &>(*pm);
		j["kind"] = "photo";
		put_photo(j, m.photo_.get());
		put_flag(j, "live_photo", m.video_ != nullptr);
		break;
	}
	case td_api::pollMediaVideo::ID: {
		auto &m = static_cast<const td_api::pollMediaVideo &>(*pm);
		j["kind"] = "video";
		put_video(j, m.video_.get());
		put_num(j, "start_timestamp", m.start_timestamp_);
		put_flag(j, "cover", m.cover_ != nullptr);
		break;
	}
	case td_api::pollMediaAnimation::ID: {
		auto &m = static_cast<const td_api::pollMediaAnimation &>(*pm);
		j["kind"] = "animation";
		if (auto *a = m.animation_.get()) {
			put_num(j, "width", a->width_);
			put_num(j, "height", a->height_);
			put_num(j, "duration", a->duration_);
			put_str(j, "mime", a->mime_type_);
		}
		break;
	}
	case td_api::pollMediaAudio::ID: {
		auto &m = static_cast<const td_api::pollMediaAudio &>(*pm);
		j["kind"] = "audio";
		if (auto *a = m.audio_.get()) {
			put_num(j, "duration", a->duration_);
			put_str(j, "title", a->title_);
			put_str(j, "performer", a->performer_);
			put_str(j, "mime", a->mime_type_);
		}
		break;
	}
	case td_api::pollMediaDocument::ID: {
		auto &m = static_cast<const td_api::pollMediaDocument &>(*pm);
		j["kind"] = "document";
		if (m.document_)
			put_str(j, "mime", m.document_->mime_type_);
		break;
	}
	case td_api::pollMediaSticker::ID: {
		auto &m = static_cast<const td_api::pollMediaSticker &>(*pm);
		j["kind"] = "sticker";
		if (m.sticker_) {
			put_str(j, "emoji", m.sticker_->emoji_);
			put_id(j, "set_id", m.sticker_->set_id_);
		}
		break;
	}
	case td_api::pollMediaLocation::ID:
		j["kind"] = "location";
		put_location(j, static_cast<const td_api::pollMediaLocation &>(*pm)
				   .location_.get());
		break;
	case td_api::pollMediaVenue::ID: {
		auto &m = static_cast<const td_api::pollMediaVenue &>(*pm);
		j["kind"] = "venue";
		if (auto *v = m.venue_.get()) {
			put_location(j, v->location_.get());
			put_str(j, "title", v->title_);
			put_str(j, "address", v->address_);
		}
		break;
	}
	case td_api::pollMediaLink::ID: {
		auto &m = static_cast<const td_api::pollMediaLink &>(*pm);
		j["kind"] = "link";
		put_str(j, "url", m.url_);
		if (m.link_preview_)
			put_str(j, "title", m.link_preview_->title_);
		break;
	}
	default:
		j["kind"] = "unsupported";
		break;
	}
	return j;
}

/* The file of a poll media, or nullptr for a location, venue or link. */
const td_api::file *poll_media_file(const td_api::PollMedia *pm,
				    const char **category,
				    std::string *file_name)
{
	if (!pm)
		return nullptr;
	switch (pm->get_id()) {
	case td_api::pollMediaPhoto::ID:
		*category = "photo";
		return photo_best_size(
			static_cast<const td_api::pollMediaPhoto &>(*pm).photo_.get());
	case td_api::pollMediaVideo::ID: {
		auto &m = static_cast<const td_api::pollMediaVideo &>(*pm);
		*category = "video";
		if (!m.video_)
			return nullptr;
		*file_name = m.video_->file_name_;
		return m.video_->video_.get();
	}
	case td_api::pollMediaAnimation::ID: {
		auto &m = static_cast<const td_api::pollMediaAnimation &>(*pm);
		*category = "animation";
		if (!m.animation_)
			return nullptr;
		*file_name = m.animation_->file_name_;
		return m.animation_->animation_.get();
	}
	case td_api::pollMediaAudio::ID: {
		auto &m = static_cast<const td_api::pollMediaAudio &>(*pm);
		*category = "audio";
		if (!m.audio_)
			return nullptr;
		*file_name = m.audio_->file_name_;
		return m.audio_->audio_.get();
	}
	case td_api::pollMediaDocument::ID: {
		auto &m = static_cast<const td_api::pollMediaDocument &>(*pm);
		*category = "document";
		if (!m.document_)
			return nullptr;
		*file_name = m.document_->file_name_;
		return m.document_->document_.get();
	}
	case td_api::pollMediaSticker::ID: {
		auto &m = static_cast<const td_api::pollMediaSticker &>(*pm);
		*category = "sticker";
		return m.sticker_ ? m.sticker_->sticker_.get() : nullptr;
	}
	default:
		return nullptr;
	}
}

/* The photo or video file of one paid media item, or nullptr for a preview
 * (not paid for: only its size is known) or an unsupported item. */
const td_api::file *paid_media_file(const td_api::PaidMedia *pm,
				    const char **category,
				    std::string *file_name)
{
	if (!pm)
		return nullptr;
	switch (pm->get_id()) {
	case td_api::paidMediaPhoto::ID:
		*category = "photo";
		return photo_best_size(
			static_cast<const td_api::paidMediaPhoto &>(*pm)
				.photo_.get());
	case td_api::paidMediaVideo::ID: {
		auto &v = static_cast<const td_api::paidMediaVideo &>(*pm);
		*category = "video";
		if (!v.video_)
			return nullptr;
		*file_name = v.video_->file_name_;
		return v.video_->video_.get();
	}
	default:
		return nullptr;
	}
}

/*
 * The file of a video's (an animation's, a video note's, a video
 * sticker's) TDLib thumbnail when it is a still image, the picture the
 * web shows before the video plays; nullptr otherwise. Thumbnails in
 * MPEG-4, GIF, TGS or WebM are moving pictures themselves, which a page
 * cannot draw as a still, so they are not stored.
 */
const td_api::file *still_thumbnail(const td_api::thumbnail *t)
{
	if (!t || !t->file_ || !t->format_)
		return nullptr;
	switch (t->format_->get_id()) {
	case td_api::thumbnailFormatJpeg::ID:
	case td_api::thumbnailFormatPng::ID:
	case td_api::thumbnailFormatWebp::ID:
		return t->file_.get();
	default:
		return nullptr;
	}
}

} /* namespace */

const char *message_content_class_name(std::int32_t id)
{
	switch (id) {
#define TGLOGGERD_NAME_CASE(cls, kind) \
	case td_api::cls::ID: return #cls;
	TGLOGGERD_MESSAGE_CONTENT_CLASSES(TGLOGGERD_NAME_CASE)
#undef TGLOGGERD_NAME_CASE
	default:
		return nullptr;
	}
}

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
	const int32_t id = content.get_id();
	const char *cls = message_content_class_name(id);

	out.content_type = content_kind(id);
	out.tdlib_type = cls ? std::string(cls) : tdlib_class_from_json(content);

	switch (id) {
	case td_api::messageAnimatedEmoji::ID: {
		/*
		 * A text message of one emoji, which TDLib presents with an
		 * animation: store it as the text it is (decision
		 * 2026-11-18-msg-animated-emoji-text). The animation adds
		 * nothing worth keeping.
		 */
		auto &c = static_cast<const td_api::messageAnimatedEmoji &>(
			content);
		if (!c.emoji_.empty())
			out.text = c.emoji_;
		return;
	}
	case td_api::messageExpiredPhoto::ID:
	case td_api::messageExpiredVideo::ID:
	case td_api::messageExpiredVideoNote::ID:
	case td_api::messageExpiredVoiceNote::ID:
		/* Nothing is left to keep but the class. */
		out.is_expired = true;
		return;
	case td_api::messageRichMessage::ID: {
		/*
		 * The blocks' plain text, for search and the preview line,
		 * and the curated summary beside TDLib's JSON (message-types.md
		 * section 4.2).
		 */
		auto &c = static_cast<const td_api::messageRichMessage &>(
			content);
		if (c.message_) {
			std::string text = rich_message_text(*c.message_);
			if (!text.empty())
				out.text = std::move(text);
		}
		out.content_extra = rich_extra_json(c);
		return;
	}
	default:
		break;
	}

	set_formatted_text(content_formatted_text(content), out);

	if (out.content_type == models::MessageContentType::Service) {
		std::string service_type;
		std::optional<std::string> text =
			describe_service_message(content,
						 msg ? sender_user_id(*msg) : 0,
						 service_type);
		if (!text.has_value()) {
			service_type = service_type_name(
				out.tdlib_type.value_or(""));
			text = generic_service_description(service_type);
		}
		out.service_type = std::move(service_type);
		if (!text->empty())
			out.text = std::move(*text);
	}

	/*
	 * messageText is stored in full by the text, entities and link
	 * preview, and the media, places, people, games, calls and service
	 * messages by their curated keys (message-types.md sections 4.2.1 to
	 * 4.2.3); every other class keeps what it carries as TDLib's JSON
	 * until a curated mapping replaces it (section 4.2).
	 */
	bool curated = false;
	std::optional<std::string> extra = media_extra_json(content, curated);
	if (!curated)
		extra = structured_extra_json(content, curated);
	if (!curated && out.content_type == models::MessageContentType::Service) {
		extra = service_extra_json(content);
		curated = true;
	}
	if (curated)
		out.content_extra = std::move(extra);
	else if (id != td_api::messageText::ID)
		out.content_extra = tdlib_extra_json(content);
}

std::string rich_message_text(const td_api::richMessage &m)
{
	std::vector<std::string> lines;
	page_blocks_text(m.blocks_, lines);
	std::string out;
	for (const auto &l : lines) {
		if (!out.empty())
			out += '\n';
		out += l;
	}
	return out;
}

std::vector<ExtraFile> rich_message_files(const td_api::richMessage &m)
{
	std::vector<ExtraFile> out;
	page_blocks_files(m.blocks_, out);
	return out;
}

const ExtraFile *rich_file_with_unique_id(const std::vector<ExtraFile> &files,
					  const std::string &unique_id)
{
	if (unique_id.empty())
		return nullptr;
	for (const auto &x : files)
		if (x.file && x.file->remote_ &&
		    x.file->remote_->unique_id_ == unique_id)
			return &x;
	return nullptr;
}

/* The message's text/caption formattedText (carrying its entities), or nullptr
 * for content that has none. Mirrors extract_message_content's text sources. */
const td_api::formattedText *
message_formatted_text(const td_api::message &message)
{
	if (!message.content_)
		return nullptr;
	return content_formatted_text(*message.content_);
}

const td_api::file *photo_best_size(const td_api::photo *p)
{
	if (!p)
		return nullptr;
	const td_api::file *best = nullptr;
	int64_t best_px = -1;
	for (const auto &sz : p->sizes_) {
		if (!sz || !sz->photo_)
			continue;
		const int64_t px = (int64_t)sz->width_ * sz->height_;
		if (px > best_px) {
			best_px = px;
			best = sz->photo_.get();
		}
	}
	return best;
}

const td_api::file *chat_photo_best_size(const td_api::chatPhoto *p)
{
	if (!p)
		return nullptr;
	const td_api::file *best = nullptr;
	int64_t best_px = -1;
	for (const auto &sz : p->sizes_) {
		if (!sz || !sz->photo_)
			continue;
		const int64_t px = (int64_t)sz->width_ * sz->height_;
		if (px > best_px) {
			best_px = px;
			best = sz->photo_.get();
		}
	}
	return best;
}

/* @s's file as a message file of @role at @position, when there is one. */
static void add_sticker_file(std::vector<ExtraFile> &out,
			     const td_api::sticker *s, const char *role,
			     int position)
{
	if (s && s->sticker_)
		out.push_back(ExtraFile{ s->sticker_.get(), "sticker", "",
					 role, position });
}

/* A regular gift's sticker, role "gift" at position 0. */
static void add_gift_sticker(std::vector<ExtraFile> &out, const td_api::gift *g)
{
	if (g)
		add_sticker_file(out, g->sticker_.get(), "gift", 0);
}

/* An upgraded gift's model sticker (role "gift", position 0) and symbol
 * sticker (position 1); each keeps its position when the other is
 * missing. */
static void add_upgraded_gift_stickers(std::vector<ExtraFile> &out,
				       const td_api::upgradedGift *g)
{
	if (!g)
		return;
	if (g->model_)
		add_sticker_file(out, g->model_->sticker_.get(), "gift", 0);
	if (g->symbol_)
		add_sticker_file(out, g->symbol_->sticker_.get(), "gift", 1);
}

/*
 * The video of an animated chat photo: the big animation (up to
 * 1280x1280), else the small one (160x160) when TDLib gives only that;
 * nullptr for a still photo.
 */
const td_api::file *chat_photo_animation(const td_api::chatPhoto *p)
{
	if (!p)
		return nullptr;
	if (p->animation_ && p->animation_->file_)
		return p->animation_->file_.get();
	if (p->small_animation_ && p->small_animation_->file_)
		return p->small_animation_->file_.get();
	return nullptr;
}

/*
 * The files of @content beyond the one message_content_file() returns, in
 * a stable order: a video's cover, a live photo's video, each paid media
 * item, a game's photo and animation, an invoice's photo and paid media,
 * the photo of a chat photo change or suggestion and its video when it
 * is animated, a chat background's document (the wallpaper or the
 * pattern), the still thumbnail of a video, an animation, a video note
 * or a video sticker (the picture shown before it plays), the media in
 * a rich message's page blocks (rich_message_files), a text message's
 * link preview's own video, animation or audio
 * (link_preview_media_files), and the stickers of gift, gifted Premium,
 * Stars, grams and giveaway messages (role "gift"). Other thumbnails and
 * a video's other qualities are not listed: the first only repeat the
 * media, the second are the same video again.
 */
std::vector<ExtraFile> message_extra_files(const td_api::MessageContent &content)
{
	std::vector<ExtraFile> out;
	auto add = [&out](const td_api::file *f, const char *category,
			  std::string name, const char *role, int position) {
		if (f)
			out.push_back(ExtraFile{ f, category, std::move(name),
						 role, position });
	};

	switch (content.get_id()) {
	case td_api::messagePhoto::ID: {
		auto &c = static_cast<const td_api::messagePhoto &>(content);
		if (c.video_)
			add(c.video_->video_.get(), "video",
			    c.video_->file_name_, "live_video", 0);
		break;
	}
	case td_api::messageVideo::ID: {
		auto &c = static_cast<const td_api::messageVideo &>(content);
		add(photo_best_size(c.cover_.get()), "photo", "", "cover", 0);
		if (c.video_)
			add(still_thumbnail(c.video_->thumbnail_.get()),
			    "thumbnail", "", "thumbnail", 0);
		break;
	}
	case td_api::messageAnimation::ID: {
		auto &c = static_cast<const td_api::messageAnimation &>(content);
		if (c.animation_)
			add(still_thumbnail(c.animation_->thumbnail_.get()),
			    "thumbnail", "", "thumbnail", 0);
		break;
	}
	case td_api::messageVideoNote::ID: {
		auto &c = static_cast<const td_api::messageVideoNote &>(content);
		if (c.video_note_)
			add(still_thumbnail(c.video_note_->thumbnail_.get()),
			    "thumbnail", "", "thumbnail", 0);
		break;
	}
	case td_api::messageSticker::ID: {
		/* Only a video sticker: a .webp one is an image itself, and
		 * a .tgs one is drawn by the page (Lottie). */
		auto &c = static_cast<const td_api::messageSticker &>(content);
		if (c.sticker_ && c.sticker_->format_ &&
		    c.sticker_->format_->get_id() ==
			    td_api::stickerFormatWebm::ID)
			add(still_thumbnail(c.sticker_->thumbnail_.get()),
			    "thumbnail", "", "thumbnail", 0);
		break;
	}
	case td_api::messagePaidMedia::ID: {
		auto &c = static_cast<const td_api::messagePaidMedia &>(content);
		for (size_t i = 0; i < c.media_.size(); i++) {
			const char *category = "unknown";
			std::string name;
			const td_api::file *f = paid_media_file(
				c.media_[i].get(), &category, &name);
			add(f, category, name, "paid", (int)i);
		}
		break;
	}
	case td_api::messageGame::ID: {
		auto &c = static_cast<const td_api::messageGame &>(content);
		if (!c.game_)
			break;
		add(photo_best_size(c.game_->photo_.get()), "photo", "",
		    "game", 0);
		if (c.game_->animation_)
			add(c.game_->animation_->animation_.get(), "animation",
			    c.game_->animation_->file_name_, "game", 1);
		break;
	}
	case td_api::messageInvoice::ID: {
		auto &c = static_cast<const td_api::messageInvoice &>(content);
		if (c.product_info_)
			add(photo_best_size(c.product_info_->photo_.get()),
			    "photo", "", "invoice", 0);
		const char *category = "unknown";
		std::string name;
		const td_api::file *f = paid_media_file(c.paid_media_.get(),
							&category, &name);
		add(f, category, name, "paid", 0);
		break;
	}
	case td_api::messagePoll::ID: {
		/*
		 * Role "poll": the poll's own media at position 0, a quiz
		 * explanation's at 1, and option i's at 2 + i.
		 */
		auto &c = static_cast<const td_api::messagePoll &>(content);
		auto add_media = [&add](const td_api::PollMedia *pm, int pos) {
			const char *category = "unknown";
			std::string name;
			const td_api::file *f = poll_media_file(pm, &category,
								&name);
			add(f, category, name, "poll", pos);
		};
		add_media(c.media_.get(), 0);
		if (!c.poll_)
			break;
		if (c.poll_->type_ &&
		    c.poll_->type_->get_id() == td_api::pollTypeQuiz::ID)
			add_media(static_cast<const td_api::pollTypeQuiz &>(
					  *c.poll_->type_).explanation_media_.get(),
				  1);
		for (size_t i = 0; i < c.poll_->options_.size(); i++)
			if (c.poll_->options_[i])
				add_media(c.poll_->options_[i]->media_.get(),
					  2 + (int)i);
		break;
	}
	case td_api::messageChatChangePhoto::ID: {
		auto *p = static_cast<const td_api::messageChatChangePhoto &>(
				  content).photo_.get();
		add(chat_photo_best_size(p), "photo", "", "chat_photo", 0);
		add(chat_photo_animation(p), "animation", "", "chat_photo", 1);
		break;
	}
	case td_api::messageSuggestProfilePhoto::ID: {
		auto *p = static_cast<const td_api::messageSuggestProfilePhoto &>(
				  content).photo_.get();
		add(chat_photo_best_size(p), "photo", "", "chat_photo", 0);
		add(chat_photo_animation(p), "animation", "", "chat_photo", 1);
		break;
	}
	case td_api::messageChatSetBackground::ID: {
		/* A wallpaper's JPEG or a pattern's PNG or TGV; a fill or
		 * a chat theme has no document. */
		auto &c = static_cast<const td_api::messageChatSetBackground &>(
			content);
		const td_api::document *d =
			c.background_ && c.background_->background_
				? c.background_->background_->document_.get()
				: nullptr;
		if (d)
			add(d->document_.get(), "document", d->file_name_,
			    "background", 0);
		break;
	}
	case td_api::messageRichMessage::ID: {
		auto &c = static_cast<const td_api::messageRichMessage &>(content);
		if (c.message_)
			out = rich_message_files(*c.message_);
		break;
	}
	case td_api::messageText::ID: {
		auto &c = static_cast<const td_api::messageText &>(content);
		if (c.link_preview_)
			out = link_preview_media_files(*c.link_preview_);
		break;
	}
	/*
	 * Role "gift": the gift's sticker, or an upgraded gift's model
	 * sticker, at position 0 and an upgraded gift's symbol sticker at
	 * 1; the animation sticker of a gifted Premium, Stars, grams or
	 * giveaway prize, and a giveaway's prize sticker, at 0.
	 */
	case td_api::messageGift::ID:
		add_gift_sticker(out, static_cast<const td_api::messageGift &>(
			content).gift_.get());
		break;
	case td_api::messageRefundedUpgradedGift::ID:
		add_gift_sticker(out,
			static_cast<const td_api::messageRefundedUpgradedGift &>(
				content).gift_.get());
		break;
	case td_api::messageUpgradedGift::ID:
		add_upgraded_gift_stickers(out,
			static_cast<const td_api::messageUpgradedGift &>(
				content).gift_.get());
		break;
	case td_api::messageUpgradedGiftPurchaseOffer::ID:
		add_upgraded_gift_stickers(out,
			static_cast<const td_api::messageUpgradedGiftPurchaseOffer &>(
				content).gift_.get());
		break;
	case td_api::messageUpgradedGiftPurchaseOfferRejected::ID:
		add_upgraded_gift_stickers(out,
			static_cast<const td_api::messageUpgradedGiftPurchaseOfferRejected &>(
				content).gift_.get());
		break;
	case td_api::messageGiftedPremium::ID:
		add_sticker_file(out, static_cast<const td_api::messageGiftedPremium &>(
			content).sticker_.get(), "gift", 0);
		break;
	case td_api::messageGiftedStars::ID:
		add_sticker_file(out, static_cast<const td_api::messageGiftedStars &>(
			content).sticker_.get(), "gift", 0);
		break;
	case td_api::messageGiftedGrams::ID:
		add_sticker_file(out, static_cast<const td_api::messageGiftedGrams &>(
			content).sticker_.get(), "gift", 0);
		break;
	case td_api::messageGiveawayPrizeStars::ID:
		add_sticker_file(out, static_cast<const td_api::messageGiveawayPrizeStars &>(
			content).sticker_.get(), "gift", 0);
		break;
	case td_api::messageGiveaway::ID:
		add_sticker_file(out, static_cast<const td_api::messageGiveaway &>(
			content).sticker_.get(), "gift", 0);
		break;
	default:
		break;
	}
	return out;
}

std::vector<ExtraFile> link_preview_media_files(const td_api::linkPreview &lp)
{
	std::vector<ExtraFile> out;
	auto add = [&out](const td_api::file *f, const char *category,
			  const std::string &name) {
		if (f)
			out.push_back(ExtraFile{ f, category, name,
						 "preview_media", 0 });
	};
	const td_api::LinkPreviewType *t = lp.type_.get();
	if (!t)
		return out;

	switch (t->get_id()) {
	case td_api::linkPreviewTypeVideo::ID: {
		auto &v = static_cast<const td_api::linkPreviewTypeVideo &>(*t);
		if (v.video_)
			add(v.video_->video_.get(), "video",
			    v.video_->file_name_);
		break;
	}
	case td_api::linkPreviewTypeEmbeddedVideoPlayer::ID: {
		auto &v = static_cast<
			const td_api::linkPreviewTypeEmbeddedVideoPlayer &>(*t);
		if (v.video_)
			add(v.video_->video_.get(), "video",
			    v.video_->file_name_);
		break;
	}
	case td_api::linkPreviewTypeAnimation::ID: {
		auto &a = static_cast<const td_api::linkPreviewTypeAnimation &>(*t);
		if (a.animation_)
			add(a.animation_->animation_.get(), "animation",
			    a.animation_->file_name_);
		break;
	}
	case td_api::linkPreviewTypeEmbeddedAnimationPlayer::ID: {
		auto &a = static_cast<
			const td_api::linkPreviewTypeEmbeddedAnimationPlayer &>(*t);
		if (a.animation_)
			add(a.animation_->animation_.get(), "animation",
			    a.animation_->file_name_);
		break;
	}
	case td_api::linkPreviewTypeAudio::ID: {
		auto &a = static_cast<const td_api::linkPreviewTypeAudio &>(*t);
		if (a.audio_)
			add(a.audio_->audio_.get(), "audio",
			    a.audio_->file_name_);
		break;
	}
	case td_api::linkPreviewTypeEmbeddedAudioPlayer::ID: {
		auto &a = static_cast<
			const td_api::linkPreviewTypeEmbeddedAudioPlayer &>(*t);
		if (a.audio_)
			add(a.audio_->audio_.get(), "audio",
			    a.audio_->file_name_);
		break;
	}
	default:
		break;
	}
	return out;
}

namespace {

bool all_digits(const std::string &s)
{
	if (s.empty() || s.size() > 18)
		return false;
	for (char ch : s)
		if (ch < '0' || ch > '9')
			return false;
	return true;
}

bool is_username(const std::string &s)
{
	if (s.size() < 4 || s.size() > 32)
		return false;
	for (char ch : s)
		if (!((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		      (ch >= '0' && ch <= '9') || ch == '_'))
			return false;
	return true;
}

std::string ascii_lower(std::string s)
{
	for (char &ch : s)
		if (ch >= 'A' && ch <= 'Z')
			ch = (char)(ch - 'A' + 'a');
	return s;
}

/* The value of @key in the query string @q ("a=1&b=2"), or "". */
std::string query_param(const std::string &q, const std::string &key)
{
	size_t i = 0;
	while (i <= q.size()) {
		size_t amp = q.find('&', i);
		if (amp == std::string::npos)
			amp = q.size();
		const std::string kv = q.substr(i, amp - i);
		const size_t eq = kv.find('=');
		if (eq != std::string::npos && kv.substr(0, eq) == key)
			return kv.substr(eq + 1);
		i = amp + 1;
	}
	return std::string();
}

/* The chat id TDLib gives the channel or supergroup of a t.me/c/ link. */
int64_t channel_chat_id(const std::string &digits)
{
	return -1000000000000LL - std::stoll(digits);
}

} /* namespace */

bool telegram_message_link_target(const std::string &url,
				  MessageLinkTarget &out)
{
	std::string rest = url;
	const std::string lower = ascii_lower(url);
	std::string path, query;

	auto split = [&path, &query](const std::string &r) {
		size_t end = r.find('#');
		std::string s = r.substr(0, end);
		size_t q = s.find('?');
		path = s.substr(0, q);
		query = q == std::string::npos ? std::string()
					       : s.substr(q + 1);
	};

	if (lower.rfind("tg://", 0) == 0) {
		split(rest.substr(5));
		const std::string what = ascii_lower(path);
		const std::string post = query_param(query, "post");
		if (!all_digits(post))
			return false;
		if (what == "resolve") {
			const std::string domain = query_param(query, "domain");
			if (!is_username(domain))
				return false;
			out = MessageLinkTarget{ 0, domain, std::stoll(post) };
			return true;
		}
		if (what == "privatepost") {
			const std::string ch = query_param(query, "channel");
			if (!all_digits(ch))
				return false;
			out = MessageLinkTarget{ channel_chat_id(ch), "",
						 std::stoll(post) };
			return true;
		}
		return false;
	}

	for (const char *scheme : { "https://", "http://" })
		if (lower.rfind(scheme, 0) == 0) {
			rest = rest.substr(strlen(scheme));
			break;
		}
	const size_t slash = rest.find('/');
	if (slash == std::string::npos)
		return false;
	std::string host = ascii_lower(rest.substr(0, slash));
	if (host.rfind("www.", 0) == 0)
		host = host.substr(4);
	if (host != "t.me" && host != "telegram.me" && host != "telegram.dog")
		return false;
	split(rest.substr(slash + 1));

	std::vector<std::string> seg;
	size_t i = 0;
	while (i <= path.size()) {
		size_t e = path.find('/', i);
		if (e == std::string::npos)
			e = path.size();
		if (e > i)
			seg.push_back(path.substr(i, e - i));
		i = e + 1;
	}

	/* t.me/c/<channel>/<message>, or .../<topic>/<message>. */
	if ((seg.size() == 3 || seg.size() == 4) && seg[0] == "c") {
		if (!all_digits(seg[1]) || !all_digits(seg.back()) ||
		    (seg.size() == 4 && !all_digits(seg[2])))
			return false;
		out = MessageLinkTarget{ channel_chat_id(seg[1]), "",
					 std::stoll(seg.back()) };
		return true;
	}
	/* t.me/<username>/<message>, or .../<topic>/<message>. */
	if ((seg.size() == 2 || seg.size() == 3) && is_username(seg[0])) {
		if (!all_digits(seg.back()) ||
		    (seg.size() == 3 && !all_digits(seg[1])))
			return false;
		out = MessageLinkTarget{ 0, seg[0], std::stoll(seg.back()) };
		return true;
	}
	return false;
}

std::optional<std::string> link_preview_extra_json(const td_api::linkPreview &lp)
{
	nlohmann::json j = nlohmann::json::object();
	const td_api::LinkPreviewType *t = lp.type_.get();
	if (!t)
		return std::nullopt;

	auto put_audio = [](nlohmann::json &o, const td_api::audio *a) {
		if (!a)
			return;
		put_num(o, "duration", a->duration_);
		put_str(o, "mime", a->mime_type_);
		put_str(o, "audio_title", a->title_);
		put_str(o, "performer", a->performer_);
		put_str(o, "file_name", a->file_name_);
	};
	auto put_animation = [](nlohmann::json &o, const td_api::animation *a) {
		if (!a)
			return;
		put_num(o, "width", a->width_);
		put_num(o, "height", a->height_);
		put_num(o, "duration", a->duration_);
		put_str(o, "mime", a->mime_type_);
		put_minithumbnail(o, a->minithumbnail_.get());
	};
	/* An embedded player's own size and duration, and its cached
	 * medium's, which can differ, under "media". */
	auto put_player = [&j](const std::string &url, int32_t w, int32_t h,
			       int32_t duration) {
		put_str(j, "embed_url", url);
		put_num(j, "embed_width", w);
		put_num(j, "embed_height", h);
		put_num(j, "duration", duration);
	};
	auto put_media = [&j](nlohmann::json m) {
		if (!m.empty())
			j["media"] = std::move(m);
	};

	switch (t->get_id()) {
	case td_api::linkPreviewTypeArticle::ID:
		put_photo(j, static_cast<const td_api::linkPreviewTypeArticle &>(
				     *t).photo_.get());
		break;
	case td_api::linkPreviewTypePhoto::ID:
		put_photo(j, static_cast<const td_api::linkPreviewTypePhoto &>(
				     *t).photo_.get());
		break;
	case td_api::linkPreviewTypeApp::ID:
		put_photo(j, static_cast<const td_api::linkPreviewTypeApp &>(
				     *t).photo_.get());
		break;
	case td_api::linkPreviewTypeWebApp::ID:
		put_photo(j, static_cast<const td_api::linkPreviewTypeWebApp &>(
				     *t).photo_.get());
		break;
	case td_api::linkPreviewTypeEmbeddedVideoPlayer::ID: {
		auto &v = static_cast<
			const td_api::linkPreviewTypeEmbeddedVideoPlayer &>(*t);
		put_player(v.url_, v.width_, v.height_, v.duration_);
		nlohmann::json m = nlohmann::json::object();
		put_video(m, v.video_.get());
		put_media(std::move(m));
		break;
	}
	case td_api::linkPreviewTypeEmbeddedAnimationPlayer::ID: {
		auto &a = static_cast<
			const td_api::linkPreviewTypeEmbeddedAnimationPlayer &>(*t);
		put_player(a.url_, a.width_, a.height_, a.duration_);
		nlohmann::json m = nlohmann::json::object();
		put_animation(m, a.animation_.get());
		put_media(std::move(m));
		break;
	}
	case td_api::linkPreviewTypeEmbeddedAudioPlayer::ID: {
		auto &a = static_cast<
			const td_api::linkPreviewTypeEmbeddedAudioPlayer &>(*t);
		put_player(a.url_, a.width_, a.height_, a.duration_);
		nlohmann::json m = nlohmann::json::object();
		put_audio(m, a.audio_.get());
		put_media(std::move(m));
		break;
	}
	case td_api::linkPreviewTypeVideo::ID: {
		auto &v = static_cast<const td_api::linkPreviewTypeVideo &>(*t);
		put_video(j, v.video_.get());
		if (v.video_)
			put_str(j, "file_name", v.video_->file_name_);
		put_num(j, "start_timestamp", v.start_timestamp_);
		put_flag(j, "cover", v.cover_ != nullptr);
		break;
	}
	case td_api::linkPreviewTypeAnimation::ID: {
		auto &a = static_cast<const td_api::linkPreviewTypeAnimation &>(*t);
		put_animation(j, a.animation_.get());
		if (a.animation_)
			put_str(j, "file_name", a.animation_->file_name_);
		break;
	}
	case td_api::linkPreviewTypeAudio::ID:
		put_audio(j, static_cast<const td_api::linkPreviewTypeAudio &>(
				     *t).audio_.get());
		break;
	case td_api::linkPreviewTypeDocument::ID: {
		auto &d = static_cast<const td_api::linkPreviewTypeDocument &>(*t);
		if (d.document_) {
			put_str(j, "mime", d.document_->mime_type_);
			put_str(j, "file_name", d.document_->file_name_);
		}
		break;
	}
	case td_api::linkPreviewTypeSticker::ID: {
		auto &s = static_cast<const td_api::linkPreviewTypeSticker &>(*t);
		if (auto *st = s.sticker_.get()) {
			put_str(j, "emoji", st->emoji_);
			put_id(j, "sticker_id", st->id_);
			put_id(j, "set_id", st->set_id_);
			put_str(j, "format",
				sticker_format_name(st->format_.get()));
			put_num(j, "width", st->width_);
			put_num(j, "height", st->height_);
		}
		break;
	}
	case td_api::linkPreviewTypeChat::ID: {
		auto &c = static_cast<const td_api::linkPreviewTypeChat &>(*t);
		if (c.type_) {
			switch (c.type_->get_id()) {
			case td_api::inviteLinkChatTypeBasicGroup::ID:
				j["chat_type"] = "basic_group";
				break;
			case td_api::inviteLinkChatTypeSupergroup::ID:
				j["chat_type"] = "supergroup";
				break;
			case td_api::inviteLinkChatTypeChannel::ID:
				j["chat_type"] = "channel";
				break;
			default:
				break;
			}
		}
		put_flag(j, "creates_join_request", c.creates_join_request_);
		break;
	}
	case td_api::linkPreviewTypeUser::ID:
		put_flag(j, "is_bot",
			 static_cast<const td_api::linkPreviewTypeUser &>(*t)
				 .is_bot_);
		break;
	case td_api::linkPreviewTypeVideoChat::ID: {
		auto &v = static_cast<const td_api::linkPreviewTypeVideoChat &>(*t);
		put_flag(j, "is_live_stream", v.is_live_stream_);
		put_flag(j, "joins_as_speaker", v.joins_as_speaker_);
		break;
	}
	case td_api::linkPreviewTypeMessage::ID: {
		/* TDLib names no target: it is the message the link names. */
		MessageLinkTarget tg;
		if (telegram_message_link_target(lp.url_, tg)) {
			put_num(j, "chat_id", tg.chat_id);
			put_str(j, "username", tg.username);
			put_num(j, "message_id", tg.message_id);
		}
		break;
	}
	case td_api::linkPreviewTypeStory::ID: {
		auto &s = static_cast<const td_api::linkPreviewTypeStory &>(*t);
		put_num(j, "poster_chat_id", s.story_poster_chat_id_);
		put_num(j, "story_id", s.story_id_);
		break;
	}
	case td_api::linkPreviewTypeExternalVideo::ID: {
		auto &v = static_cast<const td_api::linkPreviewTypeExternalVideo &>(*t);
		put_str(j, "media_url", v.url_);
		put_str(j, "mime", v.mime_type_);
		put_num(j, "width", v.width_);
		put_num(j, "height", v.height_);
		put_num(j, "duration", v.duration_);
		break;
	}
	case td_api::linkPreviewTypeExternalAudio::ID: {
		auto &a = static_cast<const td_api::linkPreviewTypeExternalAudio &>(*t);
		put_str(j, "media_url", a.url_);
		put_str(j, "mime", a.mime_type_);
		put_num(j, "duration", a.duration_);
		break;
	}
	default:
		break;
	}
	if (j.empty())
		return std::nullopt;
	return j.dump();
}

models::Poll extract_poll(const td_api::poll &p)
{
	models::Poll out;
	out.id = p.id_;
	if (p.question_) {
		out.question = p.question_->text_;
		out.question_entities = format_entities_json(*p.question_);
	}
	out.is_anonymous = p.is_anonymous_;
	out.allows_multiple_answers = p.allows_multiple_answers_;
	out.allows_revoting = p.allows_revoting_;
	out.members_only = p.members_only_;
	if (!p.country_codes_.empty())
		out.country_codes = nlohmann::json(p.country_codes_).dump();
	out.open_period = p.open_period_;
	out.close_date = p.close_date_;
	out.is_closed = p.is_closed_;
	out.total_voter_count = p.total_voter_count_;

	nlohmann::json voters = nlohmann::json::array();
	for (const auto &s : p.recent_voter_ids_) {
		nlohmann::json v = sender_json(s.get());
		if (!v.is_null())
			voters.push_back(v);
	}
	if (!voters.empty())
		out.recent_voters = voters.dump();

	if (p.type_ && p.type_->get_id() == td_api::pollTypeQuiz::ID) {
		auto &q = static_cast<const td_api::pollTypeQuiz &>(*p.type_);
		out.is_quiz = true;
		out.correct_option_ids = nlohmann::json(q.correct_option_ids_).dump();
		if (q.explanation_ && !q.explanation_->text_.empty()) {
			out.explanation = q.explanation_->text_;
			out.explanation_entities =
				format_entities_json(*q.explanation_);
		}
		nlohmann::json em = poll_media_json(q.explanation_media_.get());
		if (!em.is_null())
			out.explanation_media = em.dump();
	}

	for (size_t i = 0; i < p.options_.size(); i++) {
		const auto &o = p.options_[i];
		if (!o)
			continue;
		models::PollOption po;
		po.option_id = o->id_;
		po.position = (int)i;
		if (o->text_) {
			po.text = o->text_->text_;
			po.entities = format_entities_json(*o->text_);
		}
		nlohmann::json m = poll_media_json(o->media_.get());
		if (!m.is_null())
			po.media = m.dump();
		po.voter_count = o->voter_count_;
		po.vote_percentage = o->vote_percentage_;
		if (o->author_) {
			if (o->author_->get_id() == td_api::messageSenderUser::ID)
				po.author_user_id = static_cast<const td_api::messageSenderUser &>(
					*o->author_).user_id_;
			else if (o->author_->get_id() == td_api::messageSenderChat::ID)
				po.author_chat_id = static_cast<const td_api::messageSenderChat &>(
					*o->author_).chat_id_;
		}
		po.addition_date = o->addition_date_;
		out.options.push_back(std::move(po));
	}
	return out;
}

models::Poll extract_poll(const td_api::messagePoll &m)
{
	models::Poll out;
	if (m.poll_)
		out = extract_poll(*m.poll_);
	nlohmann::json media = poll_media_json(m.media_.get());
	if (!media.is_null())
		out.media = media.dump();
	return out;
}

bool message_topic(const td_api::MessageTopic *t, std::string &kind,
		   int64_t &id)
{
	if (!t)
		return false;
	switch (t->get_id()) {
	case td_api::messageTopicForum::ID:
		kind = "forum";
		id = static_cast<const td_api::messageTopicForum &>(*t)
			.forum_topic_id_;
		return true;
	case td_api::messageTopicThread::ID: {
		/* A TDLib message id; stored ids are server ids. */
		int64_t root = server_msg_id(
			static_cast<const td_api::messageTopicThread &>(*t)
				.message_thread_id_);
		if (root == 0)
			return false;
		kind = "thread";
		id = root;
		return true;
	}
	case td_api::messageTopicDirectMessages::ID:
		kind = "direct";
		id = static_cast<const td_api::messageTopicDirectMessages &>(*t)
			.direct_messages_chat_topic_id_;
		return true;
	case td_api::messageTopicSavedMessages::ID:
		kind = "saved";
		id = static_cast<const td_api::messageTopicSavedMessages &>(*t)
			.saved_messages_topic_id_;
		return true;
	default:
		return false;
	}
}

namespace {

void set_creator(models::ForumTopic &t, const td_api::MessageSender *s)
{
	if (!s)
		return;
	if (s->get_id() == td_api::messageSenderUser::ID)
		t.creator_user_id =
			static_cast<const td_api::messageSenderUser &>(*s).user_id_;
	else if (s->get_id() == td_api::messageSenderChat::ID)
		t.creator_chat_id =
			static_cast<const td_api::messageSenderChat &>(*s).chat_id_;
}

void set_icon(models::ForumTopic &t, const td_api::forumTopicIcon *icon)
{
	if (!icon)
		return;
	t.icon_color = icon->color_;
	if (icon->custom_emoji_id_ != 0)
		t.icon_custom_emoji_id = icon->custom_emoji_id_;
}

} /* namespace */

models::ForumTopic extract_forum_topic(const td_api::forumTopicInfo &info)
{
	models::ForumTopic t;
	t.chat_id = info.chat_id_;
	t.topic_id = info.forum_topic_id_;
	t.name = info.name_;
	t.is_name_implicit = info.is_name_implicit_;
	set_icon(t, info.icon_.get());
	t.creation_date = info.creation_date_;
	set_creator(t, info.creator_id_.get());
	t.is_general = info.is_general_;
	t.is_closed = info.is_closed_;
	t.is_hidden = info.is_hidden_;
	return t;
}

std::optional<models::ForumTopic>
forum_topic_from_created(const td_api::message &message)
{
	if (!message.content_ ||
	    message.content_->get_id() != td_api::messageForumTopicCreated::ID ||
	    !message.topic_id_ ||
	    message.topic_id_->get_id() != td_api::messageTopicForum::ID)
		return std::nullopt;
	auto &c = static_cast<const td_api::messageForumTopicCreated &>(
		*message.content_);
	models::ForumTopic t;
	t.chat_id = message.chat_id_;
	t.topic_id = static_cast<const td_api::messageTopicForum &>(
		*message.topic_id_).forum_topic_id_;
	t.name = c.name_;
	t.is_name_implicit = c.is_name_implicit_;
	set_icon(t, c.icon_.get());
	t.creation_date = message.date_;
	set_creator(t, message.sender_id_.get());
	return t;
}

namespace {

/* A forward or reply origin (messageOrigin*) as an object. */
nlohmann::json origin_json(const td_api::MessageOrigin *o)
{
	if (!o)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	switch (o->get_id()) {
	case td_api::messageOriginUser::ID:
		j["kind"] = "user";
		j["user_id"] = static_cast<const td_api::messageOriginUser &>(*o)
			.sender_user_id_;
		break;
	case td_api::messageOriginHiddenUser::ID:
		j["kind"] = "hidden_user";
		put_str(j, "sender_name",
			static_cast<const td_api::messageOriginHiddenUser &>(*o)
				.sender_name_);
		break;
	case td_api::messageOriginChat::ID: {
		auto &x = static_cast<const td_api::messageOriginChat &>(*o);
		j["kind"] = "chat";
		j["chat_id"] = x.sender_chat_id_;
		put_str(j, "author_signature", x.author_signature_);
		break;
	}
	case td_api::messageOriginChannel::ID: {
		auto &x = static_cast<const td_api::messageOriginChannel &>(*o);
		j["kind"] = "channel";
		j["chat_id"] = x.chat_id_;
		put_msg_id(j, "message_id", x.message_id_);
		put_str(j, "author_signature", x.author_signature_);
		break;
	}
	default:
		return nullptr;
	}
	return j;
}

/*
 * A keyboard: the kind, and for a keyboard with buttons each row's buttons
 * with their text, type and what a reader needs of their target (a URL, a
 * user, a query). Callback data is the bot's own opaque payload and is not
 * kept.
 */
nlohmann::json reply_markup_json(const td_api::ReplyMarkup *rm)
{
	if (!rm)
		return nullptr;
	nlohmann::json j = nlohmann::json::object();
	switch (rm->get_id()) {
	case td_api::replyMarkupRemoveKeyboard::ID:
		j["kind"] = "remove_keyboard";
		break;
	case td_api::replyMarkupForceReply::ID:
		j["kind"] = "force_reply";
		put_str(j, "placeholder",
			static_cast<const td_api::replyMarkupForceReply &>(*rm)
				.input_field_placeholder_);
		break;
	case td_api::replyMarkupShowKeyboard::ID: {
		auto &k = static_cast<const td_api::replyMarkupShowKeyboard &>(*rm);
		j["kind"] = "keyboard";
		nlohmann::json rows = nlohmann::json::array();
		for (const auto &row : k.rows_) {
			nlohmann::json r = nlohmann::json::array();
			for (const auto &btn : row) {
				if (!btn)
					continue;
				nlohmann::json e = { { "text", btn->text_ } };
				put_str(e, "type", tl_variant(btn->type_.get(),
							      "keyboardButtonType"));
				if (btn->type_ && btn->type_->get_id() ==
					td_api::keyboardButtonTypeWebApp::ID)
					put_str(e, "url",
						static_cast<const td_api::keyboardButtonTypeWebApp &>(
							*btn->type_).url_);
				r.push_back(e);
			}
			rows.push_back(r);
		}
		j["rows"] = rows;
		put_flag(j, "one_time", k.one_time_);
		put_flag(j, "persistent", k.is_persistent_);
		put_str(j, "placeholder", k.input_field_placeholder_);
		break;
	}
	case td_api::replyMarkupInlineKeyboard::ID: {
		auto &k = static_cast<const td_api::replyMarkupInlineKeyboard &>(*rm);
		j["kind"] = "inline_keyboard";
		nlohmann::json rows = nlohmann::json::array();
		for (const auto &row : k.rows_) {
			nlohmann::json r = nlohmann::json::array();
			for (const auto &btn : row) {
				if (!btn)
					continue;
				nlohmann::json e = { { "text", btn->text_ } };
				const td_api::InlineKeyboardButtonType *t = btn->type_.get();
				put_str(e, "type", tl_variant(t, "inlineKeyboardButtonType"));
				if (t) {
					switch (t->get_id()) {
					case td_api::inlineKeyboardButtonTypeUrl::ID:
						put_str(e, "url", static_cast<const td_api::inlineKeyboardButtonTypeUrl &>(*t).url_);
						break;
					case td_api::inlineKeyboardButtonTypeLoginUrl::ID:
						put_str(e, "url", static_cast<const td_api::inlineKeyboardButtonTypeLoginUrl &>(*t).url_);
						break;
					case td_api::inlineKeyboardButtonTypeWebApp::ID:
						put_str(e, "url", static_cast<const td_api::inlineKeyboardButtonTypeWebApp &>(*t).url_);
						break;
					case td_api::inlineKeyboardButtonTypeSwitchInline::ID:
						put_str(e, "query", static_cast<const td_api::inlineKeyboardButtonTypeSwitchInline &>(*t).query_);
						break;
					case td_api::inlineKeyboardButtonTypeUser::ID:
						put_num(e, "user_id", static_cast<const td_api::inlineKeyboardButtonTypeUser &>(*t).user_id_);
						break;
					case td_api::inlineKeyboardButtonTypeCopyText::ID:
						put_str(e, "copy_text", static_cast<const td_api::inlineKeyboardButtonTypeCopyText &>(*t).text_);
						break;
					default:
						break;
					}
				}
				r.push_back(e);
			}
			rows.push_back(r);
		}
		j["rows"] = rows;
		break;
	}
	default:
		return nullptr;
	}
	return j;
}

} /* namespace */

std::optional<std::string>
formatted_text_entities_json(const td_api::formattedText &ft)
{
	return format_entities_json(ft);
}

std::optional<std::string> fact_check_json(const td_api::factCheck *fc)
{
	if (!fc || !fc->text_ || fc->text_->text_.empty())
		return std::nullopt;
	nlohmann::json j = { { "text", fc->text_->text_ } };
	if (auto e = format_entities_json(*fc->text_))
		j["entities"] = nlohmann::json::parse(*e);
	put_str(j, "country_code", fc->country_code_);
	return j.dump();
}

std::optional<std::string> message_meta_json(const td_api::message &m)
{
	nlohmann::json j = nlohmann::json::object();

	put_num(j, "via_bot_user_id", m.via_bot_user_id_);
	put_num(j, "business_bot_user_id", m.sender_business_bot_user_id_);
	put_json(j, "guest_bot_caller", sender_json(m.guest_bot_caller_id_.get()));

	if (m.reply_to_) {
		if (m.reply_to_->get_id() == td_api::messageReplyToMessage::ID) {
			auto &r = static_cast<const td_api::messageReplyToMessage &>(
				*m.reply_to_);
			nlohmann::json reply = nlohmann::json::object();
			if (r.quote_ && r.quote_->text_ &&
			    !r.quote_->text_->text_.empty()) {
				nlohmann::json q = { { "text", r.quote_->text_->text_ } };
				if (auto e = format_entities_json(*r.quote_->text_))
					q["entities"] = nlohmann::json::parse(*e);
				put_num(q, "position", r.quote_->position_);
				put_flag(q, "manual", r.quote_->is_manual_);
				reply["quote"] = q;
			}
			put_num(reply, "checklist_task_id", r.checklist_task_id_);
			put_str(reply, "poll_option_id", r.poll_option_id_);
			/* A reply into a chat not logged: what the message
			 * replied to was, as far as this copy says. */
			put_json(reply, "origin", origin_json(r.origin_.get()));
			put_num(reply, "origin_send_date", r.origin_send_date_);
			if (r.content_) {
				models::MessageContent rc;
				extract_message_content(*r.content_, rc);
				nlohmann::json content = nlohmann::json::object();
				put_str(content, "tdlib_type", rc.tdlib_type.value_or(""));
				put_str(content, "text", rc.text.value_or(""));
				reply["content"] = content;
			}
			if (!reply.empty())
				j["reply"] = reply;
		} else if (m.reply_to_->get_id() == td_api::messageReplyToStory::ID) {
			auto &r = static_cast<const td_api::messageReplyToStory &>(
				*m.reply_to_);
			j["reply_to_story"] = {
				{ "poster_chat_id", r.story_poster_chat_id_ },
				{ "story_id", r.story_id_ },
			};
		}
	}

	if (m.forward_info_) {
		auto &fi = *m.forward_info_;
		if (fi.source_) {
			auto &src = *fi.source_;
			nlohmann::json fs = nlohmann::json::object();
			put_num(fs, "chat_id", src.chat_id_);
			put_msg_id(fs, "message_id", src.message_id_);
			put_json(fs, "sender", sender_json(src.sender_id_.get()));
			put_str(fs, "sender_name", src.sender_name_);
			put_num(fs, "date", src.date_);
			if (!fs.empty())
				j["forward_source"] = fs;
		}
		put_str(j, "psa_type", fi.public_service_announcement_type_);
	}

	if (m.import_info_) {
		nlohmann::json im = nlohmann::json::object();
		put_str(im, "sender_name", m.import_info_->sender_name_);
		put_num(im, "date", m.import_info_->date_);
		j["import"] = im;
	}

	put_json(j, "reply_markup", reply_markup_json(m.reply_markup_.get()));

	/*
	 * The link preview options the sender chose (turned off, another URL,
	 * small or large media, above the text). They belong to the message,
	 * not to the card: a message with previews turned off has no card.
	 * TDLib leaves them out when they are the defaults.
	 */
	if (m.content_ && m.content_->get_id() == td_api::messageText::ID) {
		auto &t = static_cast<const td_api::messageText &>(*m.content_);
		if (auto *o = t.link_preview_options_.get()) {
			nlohmann::json lo = nlohmann::json::object();
			put_flag(lo, "disabled", o->is_disabled_);
			put_str(lo, "url", o->url_);
			put_flag(lo, "force_small_media", o->force_small_media_);
			put_flag(lo, "force_large_media", o->force_large_media_);
			put_flag(lo, "show_above_text", o->show_above_text_);
			if (!lo.empty())
				j["link_preview_options"] = lo;
		}
	}
	put_id(j, "effect_id", m.effect_id_);

	if (m.self_destruct_type_) {
		if (m.self_destruct_type_->get_id() ==
		    td_api::messageSelfDestructTypeTimer::ID)
			put_num(j, "self_destruct_time",
				static_cast<const td_api::messageSelfDestructTypeTimer &>(
					*m.self_destruct_type_).self_destruct_time_);
		else
			j["self_destruct_immediately"] = true;
	}

	put_num(j, "paid_message_star_count", m.paid_message_star_count_);
	put_num(j, "sender_boost_count", m.sender_boost_count_);
	put_str(j, "sender_tag", m.sender_tag_);

	if (auto fc = fact_check_json(m.fact_check_.get()))
		j["fact_check"] = nlohmann::json::parse(*fc);

	if (m.restriction_info_ &&
	    (!m.restriction_info_->restriction_reason_.empty() ||
	     m.restriction_info_->has_sensitive_content_)) {
		nlohmann::json ri = nlohmann::json::object();
		put_str(ri, "reason", m.restriction_info_->restriction_reason_);
		put_flag(ri, "sensitive", m.restriction_info_->has_sensitive_content_);
		j["restriction"] = ri;
	}

	if (m.suggested_post_info_) {
		auto &sp = *m.suggested_post_info_;
		nlohmann::json p = nlohmann::json::object();
		put_json(p, "price", price_json(sp.price_.get()));
		put_num(p, "send_date", sp.send_date_);
		put_str(p, "state", tl_variant(sp.state_.get(), "suggestedPostState"));
		j["suggested_post"] = p;
	}

	if (j.empty())
		return std::nullopt;
	return j.dump();
}

namespace {

/* Coordinates as plain decimal degrees, enough for a map link. */
std::string coord(double v)
{
	char buf[32];
	snprintf(buf, sizeof(buf), "%.6f", v);
	return buf;
}

std::string map_link(const td_api::location *l)
{
	if (!l)
		return std::string();
	const std::string lat = coord(l->latitude_), lon = coord(l->longitude_);
	return "https://www.openstreetmap.org/?mlat=" + lat + "&mlon=" + lon +
	       "#map=17/" + lat + "/" + lon;
}

std::string duration_text(int32_t seconds)
{
	char buf[32];
	if (seconds >= 3600)
		snprintf(buf, sizeof(buf), "%d:%02d:%02d", seconds / 3600,
			 seconds / 60 % 60, seconds % 60);
	else
		snprintf(buf, sizeof(buf), "%d:%02d", seconds / 60, seconds % 60);
	return buf;
}

std::string text_of(const td_api::formattedText *ft)
{
	return ft ? ft->text_ : std::string();
}

} /* namespace */

std::string forward_summary(const td_api::MessageContent &c)
{
	switch (c.get_id()) {
	case td_api::messageLocation::ID: {
		auto &m = static_cast<const td_api::messageLocation &>(c);
		return "Location: " + map_link(m.location_.get());
	}
	case td_api::messageLiveLocation::ID: {
		auto &m = static_cast<const td_api::messageLiveLocation &>(c);
		return "Live location: " +
		       map_link(m.location_ ? m.location_->location_.get()
					    : nullptr);
	}
	case td_api::messageVenue::ID: {
		auto &m = static_cast<const td_api::messageVenue &>(c);
		if (!m.venue_)
			return "Venue";
		std::string out = "Venue: " + m.venue_->title_;
		if (!m.venue_->address_.empty())
			out += ", " + m.venue_->address_;
		std::string link = map_link(m.venue_->location_.get());
		if (!link.empty())
			out += "\n" + link;
		return out;
	}
	case td_api::messageContact::ID: {
		auto &m = static_cast<const td_api::messageContact &>(c);
		if (!m.contact_)
			return "Contact";
		std::string name = m.contact_->first_name_;
		if (!m.contact_->last_name_.empty())
			name += (name.empty() ? "" : " ") + m.contact_->last_name_;
		std::string out = "Contact: " + name;
		if (!m.contact_->phone_number_.empty())
			out += (name.empty() ? "" : ", ") + m.contact_->phone_number_;
		return out;
	}
	case td_api::messageDice::ID: {
		auto &m = static_cast<const td_api::messageDice &>(c);
		std::string out = "Dice " + m.emoji_;
		if (m.value_ != 0)
			out += ": " + std::to_string(m.value_);
		return out;
	}
	case td_api::messageStakeDice::ID: {
		auto &m = static_cast<const td_api::messageStakeDice &>(c);
		return m.value_ != 0 ? "Dice: " + std::to_string(m.value_)
				     : std::string("Dice");
	}
	case td_api::messagePoll::ID: {
		auto &m = static_cast<const td_api::messagePoll &>(c);
		if (!m.poll_)
			return "Poll";
		const bool quiz = m.poll_->type_ &&
			m.poll_->type_->get_id() == td_api::pollTypeQuiz::ID;
		std::string out = std::string(quiz ? "Quiz: " : "Poll: ") +
				  text_of(m.poll_->question_.get());
		for (const auto &o : m.poll_->options_)
			if (o)
				out += "\n- " + text_of(o->text_.get());
		return out;
	}
	case td_api::messageGame::ID: {
		auto &m = static_cast<const td_api::messageGame &>(c);
		return m.game_ ? "Game: " + m.game_->title_ : std::string("Game");
	}
	case td_api::messageStory::ID:
		return "Shared a story";
	case td_api::messageInvoice::ID: {
		auto &m = static_cast<const td_api::messageInvoice &>(c);
		std::string out = "Invoice";
		if (m.product_info_ && !m.product_info_->title_.empty())
			out += ": " + m.product_info_->title_;
		if (!m.currency_.empty())
			out += " (" + m.currency_ + ")";
		return out;
	}
	case td_api::messageChecklist::ID: {
		auto &m = static_cast<const td_api::messageChecklist &>(c);
		/* The title is the message text already. */
		std::string out = "Checklist:";
		if (m.list_)
			for (const auto &t : m.list_->tasks_)
				if (t)
					out += std::string("\n") +
					       (t->completed_by_ ? "[x] " : "[ ] ") +
					       text_of(t->text_.get());
		return out;
	}
	case td_api::messageCall::ID: {
		auto &m = static_cast<const td_api::messageCall &>(c);
		const char *what = m.is_video_ ? "Video call" : "Call";
		if (m.discard_reason_ &&
		    m.discard_reason_->get_id() == td_api::callDiscardReasonMissed::ID)
			return std::string("Missed ") +
			       (m.is_video_ ? "video call" : "call");
		if (m.discard_reason_ &&
		    m.discard_reason_->get_id() == td_api::callDiscardReasonDeclined::ID)
			return std::string("Declined ") +
			       (m.is_video_ ? "video call" : "call");
		if (m.duration_ > 0)
			return std::string(what) + ", " + duration_text(m.duration_);
		return what;
	}
	case td_api::messageGroupCall::ID: {
		auto &m = static_cast<const td_api::messageGroupCall &>(c);
		std::string out = m.is_video_ ? "Group video call" : "Group call";
		if (m.duration_ > 0)
			out += ", " + duration_text(m.duration_);
		return out;
	}
	case td_api::messageGiveaway::ID: {
		auto &m = static_cast<const td_api::messageGiveaway &>(c);
		std::string out = "Giveaway";
		if (m.winner_count_ > 0)
			out += ": " + std::to_string(m.winner_count_) +
			       (m.winner_count_ == 1 ? " winner" : " winners");
		return out;
	}
	case td_api::messagePaidMedia::ID: {
		auto &m = static_cast<const td_api::messagePaidMedia &>(c);
		std::string out = "Paid media";
		if (!m.media_.empty())
			out += ": " + std::to_string(m.media_.size()) +
			       (m.media_.size() == 1 ? " item" : " items");
		if (m.star_count_ > 0)
			out += ", " + std::to_string(m.star_count_) + " stars";
		return out;
	}
	default:
		return std::string();
	}
}

std::optional<std::string>
reactions_json(const std::vector<td_api::object_ptr<td_api::messageReaction>> &r)
{
	nlohmann::json out = nlohmann::json::array();
	for (const auto &x : r) {
		if (!x || !x->type_)
			continue;
		nlohmann::json e = nlohmann::json::object();
		switch (x->type_->get_id()) {
		case td_api::reactionTypeEmoji::ID:
			e["emoji"] = static_cast<const td_api::reactionTypeEmoji &>(
				*x->type_).emoji_;
			break;
		case td_api::reactionTypeCustomEmoji::ID:
			e["custom_emoji_id"] = std::to_string(
				static_cast<const td_api::reactionTypeCustomEmoji &>(
					*x->type_).custom_emoji_id_);
			break;
		case td_api::reactionTypePaid::ID:
			e["paid"] = true;
			break;
		default:
			continue;
		}
		e["count"] = x->total_count_;
		out.push_back(e);
	}
	if (out.empty())
		return std::nullopt;
	return out.dump();
}

std::optional<models::Interaction>
extract_interaction(const td_api::messageInteractionInfo *info)
{
	if (!info)
		return std::nullopt;
	models::Interaction ia;
	ia.view_count = info->view_count_;
	ia.forward_count = info->forward_count_;
	if (info->reply_info_)
		ia.reply_count = info->reply_info_->reply_count_;
	if (info->reactions_ && !info->reactions_->are_tags_)
		ia.reactions = reactions_json(info->reactions_->reactions_);
	/* An explicit empty list: the reactions were all withdrawn. */
	if (info->reactions_ && !info->reactions_->are_tags_ && !ia.reactions)
		ia.reactions = "[]";
	return ia;
}

const td_api::file *chat_photo_best_size_of(
	const std::vector<td_api::object_ptr<td_api::photoSize>> &sizes)
{
	const td_api::file *best = nullptr;
	int64_t best_px = -1;
	for (const auto &sz : sizes) {
		if (!sz || !sz->photo_)
			continue;
		const int64_t px = (int64_t)sz->width_ * sz->height_;
		if (px > best_px) {
			best_px = px;
			best = sz->photo_.get();
		}
	}
	return best;
}

} /* namespace tgloggerd */
