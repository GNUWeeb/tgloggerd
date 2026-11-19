-- Gift service messages and a giveaway with their stickers
-- (docs/design/message-types.md sections 3.5, 4.2.3 and 4.3), for
-- ServiceInfo.detail of kind gift and theme and GiveawayDetail.sticker.
-- Loaded after "sample", whose users give and receive the gifts and
-- whose C/C++ Study Group owns one. Invented; nothing here comes from a
-- real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000043 (Gift Lab, not exposed)
--   files      878001 - 878004
--   messages   group rows 4401 - 4412 (Gift Lab, n = 1 - 12)
-- Gift Lab's message n has message id 740000 + n and date
-- 1794931200 - 3600 + 60 * n; Dewi sends them all but n = 11, which
-- Bima sends, so a member granted Gift Lab sees both of them. Andi,
-- who never writes there, and C/C++ Study Group, which an admin hides,
-- are named by the upgraded gift only. By n:
--   1  gift from Bima to Dewi, with its sticker and a text
--   2  upgraded_gift with its model and symbol stickers, owned by C/C++
--      Study Group, upgraded from Andi's gift to Dewi
--   3  upgraded_gift_purchase_offer in grams, no sticker stored
--   4  gifted_stars from Bima to Dewi, with its sticker
--   5  giveaway_prize_stars, no sticker stored
--   6  chat_set_theme from an upgraded gift
--   7  chat_set_theme to an emoji theme
--   8  chat_set_theme reset (nothing stored)
--   9  gift logged without its detail or sticker
--  10  a giveaway with its prize sticker
--  11  a text message from Bima
--  12  refunded_upgraded_gift of a private gift (no sender)

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(878001, 'gift-878001', 'gift-u-878001', 'sticker', 20000, 1,
	 UNHEX(SHA2('gift 878001', 256)), 'tgs', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(878002, 'gift-878002', 'gift-u-878002', 'sticker', 9000, 0,
	 NULL, 'tgs', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(878003, 'gift-878003', 'gift-u-878003', 'sticker', 21000, 1,
	 UNHEX(SHA2('gift 878003', 256)), 'tgs', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(878004, 'gift-878004', 'gift-u-878004', 'sticker', 41000, 1,
	 UNHEX(SHA2('gift 878004', 256)), 'webm', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000043, 'supergroup', 'Gift Lab', 'Gifts.', 12,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra, service_type) VALUES
	(4401, -1001000000043, 740001, 9299000001, 1794927660, 'service', 'sent a gift', 'messageGift', '{"gift":{"id":"5170145012310081615","star_count":50,"emoji":"\\u2764"},"sender":{"user_id":9299000002},"receiver":{"user_id":9299000001},"received_gift_id":"r1","text":{"text":"Happy birthday"}}', 'gift'),
	(4402, -1001000000043, 740002, 9299000001, 1794927720, 'service', 'upgraded a gift', 'messageUpgradedGift', '{"gift":{"id":"77","regular_gift_id":"5170145012310081615","title":"Plush Pepe","name":"PlushPepe-42","number":42,"owner":{"chat_id":-1001000000004},"model":{"name":"Pumpkin","rarity_per_mille":15,"crafted":true},"symbol":{"name":"Star","rarity":"epic"},"backdrop":{"name":"Onyx Black","center_color":0,"edge_color":3355443,"symbol_color":16777215,"text_color":16711680,"rarity_per_mille":0},"original_details":{"sender":{"user_id":9299000004},"receiver":{"user_id":9299000001},"text":{"text":"For you"},"date":1794900000}},"sender":{"user_id":9299000001},"receiver":{"user_id":9299000002},"origin":"upgrade","gift_message_id":740001}', 'upgraded_gift'),
	(4403, -1001000000043, 740003, 9299000001, 1794927780, 'service', 'offered to buy a gift', 'messageUpgradedGiftPurchaseOffer', '{"gift":{"id":"78","title":"Plush Pepe","name":"PlushPepe-43","number":43},"state":"pending","price":{"gram_cents":1250},"expiration_date":1795000000}', 'upgraded_gift_purchase_offer'),
	(4404, -1001000000043, 740004, 9299000001, 1794927840, 'service', 'gifted Telegram Stars', 'messageGiftedStars', '{"gifter_user_id":9299000002,"receiver_user_id":9299000001,"star_count":500,"transaction_id":"t1"}', 'gifted_stars'),
	(4405, -1001000000043, 740005, 9299000001, 1794927900, 'service', 'won Telegram Stars in a giveaway', 'messageGiveawayPrizeStars', '{"star_count":1000,"transaction_id":"t2","boosted_chat_id":-1001000000043}', 'giveaway_prize_stars'),
	(4406, -1001000000043, 740006, 9299000001, 1794927960, 'service', 'changed the chat theme', 'messageChatSetTheme', '{"theme_kind":"gift","gift_id":"77","gift_name":"PlushPepe-42"}', 'chat_set_theme'),
	(4407, -1001000000043, 740007, 9299000001, 1794928020, 'service', 'changed the chat theme', 'messageChatSetTheme', '{"theme_name":"\\u2764"}', 'chat_set_theme'),
	(4408, -1001000000043, 740008, 9299000001, 1794928080, 'service', 'disabled the chat theme', 'messageChatSetTheme', NULL, 'chat_set_theme'),
	(4409, -1001000000043, 740009, 9299000001, 1794928140, 'service', 'sent a gift', NULL, NULL, 'gift'),
	(4410, -1001000000043, 740010, 9299000001, 1794928200, 'giveaway', NULL, 'messageGiveaway', '{"boosted_chat_id":-1001000000043,"winner_count":3,"prize":{"stars":1000}}', NULL),
	(4411, -1001000000043, 740011, 9299000002, 1794928260, 'text', 'Thanks!', NULL, NULL, NULL),
	(4412, -1001000000043, 740012, 9299000001, 1794928320, 'service', 'refunded a gift', 'messageRefundedUpgradedGift', '{"gift":{"id":"79","star_count":25},"receiver":{"user_id":9299000001},"origin":"upgrade"}', 'refunded_upgraded_gift');

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(4401, 'gift', 0, 878001),
	(4402, 'gift', 0, 878001),
	(4402, 'gift', 1, 878002),
	(4404, 'gift', 0, 878003),
	(4410, 'gift', 0, 878004);
