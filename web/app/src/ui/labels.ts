// Human names for the archive's message kinds: the content types and the
// service types the daemon writes, for the chat, the message page, the
// message lists and their filters.
import type { S } from '../api/schema.ts';

type ContentType = S['ContentType'];

// Every content type has a label: a new one fails to compile here.
const CONTENT: Record<ContentType, string> = {
	text: 'Text', photo: 'Photo', video: 'Video', document: 'File', audio: 'Audio', voice: 'Voice message', sticker: 'Sticker', animation: 'GIF',
	service: 'Service message', unknown: 'Unknown kind', video_note: 'Video message', location: 'Location', venue: 'Venue', contact: 'Contact',
	dice: 'Dice', game: 'Game', poll: 'Poll', checklist: 'Checklist', story: 'Story', paid_media: 'Paid media', invoice: 'Invoice',
	giveaway: 'Giveaway', call: 'Call', rich: 'Rich message', widget: 'Widget', expired: 'Expired media', unsupported: 'Unsupported message',
};

export const contentLabel = (t: ContentType): string => CONTENT[t] ?? t;

// The content type filter's labels, by value.
export const CONTENT_LABELS: Readonly<Record<string, string>> = CONTENT;

// Service types are not a closed list: the daemon names each new TDLib
// class after itself (docs/design/message-types.md section 3), so a type
// missing here shows its raw name.
const SERVICE: Readonly<Record<string, string>> = {
	chat_create: 'Group created',
	chat_change_title: 'Title changed',
	chat_change_photo: 'Photo changed',
	chat_delete_photo: 'Photo removed',
	chat_add_members: 'Members added',
	chat_join_by_link: 'Joined by link',
	chat_join_by_request: 'Join request approved',
	chat_join_from_community: 'Joined from a community',
	chat_delete_member: 'Member left or removed',
	chat_added_to_community: 'Added to a community',
	chat_removed_from_community: 'Removed from a community',
	chat_upgrade_to: 'Upgraded to a supergroup',
	chat_upgrade_from: 'Upgraded from a group',
	chat_owner_left: 'Owner left',
	chat_owner_changed: 'Owner changed',
	chat_has_protected_content_toggled: 'Content protection changed',
	chat_has_protected_content_disable_requested: 'Content protection off requested',
	pin_message: 'Message pinned',
	screenshot_taken: 'Screenshot taken',
	chat_set_background: 'Background set',
	chat_set_theme: 'Theme set',
	chat_set_message_auto_delete_time: 'Auto-delete timer set',
	chat_boost: 'Group boosted',
	forum_topic_created: 'Topic created',
	forum_topic_edited: 'Topic edited',
	forum_topic_is_closed_toggled: 'Topic closed or reopened',
	forum_topic_is_hidden_toggled: 'Topic hidden or shown',
	custom_service_action: 'Custom action',
	contact_registered: 'Contact joined Telegram',
	suggest_profile_photo: 'Profile photo suggested',
	suggest_birthdate: 'Birthday suggested',
	proximity_alert_triggered: 'Proximity alert',
	managed_bot_created: 'Managed bot created',
	bot_write_access_allowed: 'Bot allowed to write',
	users_shared: 'Users shared',
	chat_shared: 'Chat shared',
	web_app_data_sent: 'Web app data sent',
	web_app_data_received: 'Web app data received',
	passport_data_sent: 'Passport data sent',
	passport_data_received: 'Passport data received',
	game_score: 'Game score',
	video_chat_scheduled: 'Video chat scheduled',
	video_chat_started: 'Video chat started',
	video_chat_ended: 'Video chat ended',
	invite_video_chat_participants: 'Invited to a video chat',
	poll_option_added: 'Poll option added',
	poll_option_deleted: 'Poll option removed',
	checklist_tasks_done: 'Checklist tasks done',
	checklist_tasks_added: 'Checklist tasks added',
	suggested_post_approval_failed: 'Suggested post failed',
	suggested_post_approved: 'Suggested post approved',
	suggested_post_declined: 'Suggested post declined',
	suggested_post_paid: 'Suggested post paid',
	suggested_post_refunded: 'Suggested post refunded',
	payment_successful: 'Payment made',
	payment_successful_bot: 'Payment received',
	payment_refunded: 'Payment refunded',
	gifted_premium: 'Premium gifted',
	premium_gift_code: 'Premium gift code',
	gifted_stars: 'Stars gifted',
	gifted_grams: 'Grams gifted',
	giveaway_created: 'Giveaway created',
	giveaway_completed: 'Giveaway completed',
	giveaway_winners: 'Giveaway winners',
	giveaway_prize_stars: 'Giveaway prize',
	gift: 'Gift',
	upgraded_gift: 'Upgraded gift',
	refunded_upgraded_gift: 'Gift refunded',
	upgraded_gift_purchase_offer: 'Gift purchase offer',
	upgraded_gift_purchase_offer_rejected: 'Gift purchase offer rejected',
	paid_messages_refunded: 'Paid messages refunded',
	paid_message_price_changed: 'Message price changed',
	direct_message_price_changed: 'Direct message price changed',
	unpaid_messages_allowed_by_link: 'Unpaid messages allowed',
	ton_wallet_transfer: 'TON transfer',
	ton_connect_request: 'TON Connect request',
};

export const serviceLabel = (t: string): string => (Object.hasOwn(SERVICE, t) ? SERVICE[t]! : t);

// A message's kind in a few words: a service message by its action,
// when known, else by its content type.
export const typeLabel = (ct: ContentType, service?: string | null): string => (ct === 'service' && service ? serviceLabel(service) : contentLabel(ct));
