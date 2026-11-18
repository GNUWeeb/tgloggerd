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

/*
 * The files of @content beyond the one message_content_file() returns, in
 * a stable order: a video's cover, a live photo's video, each paid media
 * item, a game's photo and animation, an invoice's photo and paid media,
 * the photo of a chat photo change or suggestion. Thumbnails and a
 * video's other qualities are not listed: the first only repeat the
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
	case td_api::messageChatChangePhoto::ID:
		add(chat_photo_best_size(
			    static_cast<const td_api::messageChatChangePhoto &>(
				    content).photo_.get()),
		    "photo", "", "chat_photo", 0);
		break;
	case td_api::messageSuggestProfilePhoto::ID:
		add(chat_photo_best_size(
			    static_cast<const td_api::messageSuggestProfilePhoto &>(
				    content).photo_.get()),
		    "photo", "", "chat_photo", 0);
		break;
	default:
		break;
	}
	return out;
}

} /* namespace tgloggerd */
