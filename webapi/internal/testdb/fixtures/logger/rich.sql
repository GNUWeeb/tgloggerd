-- Rich messages (docs/design/message-types.md sections 4.2 and 4.3), for
-- the RichDetail the chat and the message page send. Loaded after
-- "sample", whose users are the sender and the people named. Invented;
-- nothing here comes from a real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000042 (Rich Lab, not exposed)
--   files      896001 - 896002
--   messages   group rows 4301 - 4302 (Rich Lab, message ids 730001 -
--              730002)
-- Its only sender is Dewi, so a member granted Rich Lab does not see
-- Bima, whom message 1 mentions. By row:
--   4301  a page of a title, a paragraph mentioning Bima, a photo
--         (two sizes; the larger is file 896001, position 0), a list
--         holding a video (896002, position 1), a table and a block of
--         a class this service does not know; edited once (its earlier
--         version: the title and the photo, not full)
--   4302  a page whose JSON was over the daemon's cap: the class and
--         curated keys only

INSERT INTO telegram_files (id, tg_file_id, tg_unique_id, file_type, file_size, on_disk,
	sha256, file_ext, orig_file_name, created_at, accessed_at) VALUES
	(896001, 'rich-896001', 'rich-u-896001', 'photo', 40000, 1,
	 UNHEX(SHA2('rich 896001', 256)), 'jpg', '', '2026-11-17 10:00:00', '2026-11-17 10:00:00'),
	(896002, 'rich-896002', 'rich-u-896002', 'video', 900000, 0,
	 NULL, 'mp4', 'clip.mp4', '2026-11-17 10:00:00', '2026-11-17 10:00:00');

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000042, 'supergroup', 'Rich Lab', 'Rich messages.', 2,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra, service_type, file_id, edit_date) VALUES
	(4301, -1001000000042, 730001, 9299000001, 1794927660, 'rich',
	 'Release notes\nThanks Bima and everyone\nThe team\n1. \nScores\nName', 'messageRichMessage',
	 '{"blocks":6,"is_rtl":false,"tdlib":{"@type":"messageRichMessage","message":{"@type":"richMessage","blocks":[{"@type":"pageBlockTitle","title":{"@type":"richTextPlain","text":"Release notes"}},{"@type":"pageBlockParagraph","text":{"@type":"richTexts","texts":[{"@type":"richTextPlain","text":"Thanks "},{"@type":"richTextMentionName","text":{"@type":"richTextPlain","text":"Bima"},"user_id":9299000002},{"@type":"richTextPlain","text":" and "},{"@type":"richTextBold","text":{"@type":"richTextPlain","text":"everyone"}}]}},{"@type":"pageBlockPhoto","photo":{"@type":"photo","has_stickers":false,"sizes":[{"@type":"photoSize","type":"m","photo":{"@type":"file","id":11,"size":1000,"expected_size":1000,"remote":{"@type":"remoteFile","id":"r11","unique_id":"rich-u-small","is_uploading_active":false,"is_uploading_completed":true,"uploaded_size":1000}},"width":320,"height":240,"progressive_sizes":[]},{"@type":"photoSize","type":"y","photo":{"@type":"file","id":12,"size":1000,"expected_size":1000,"remote":{"@type":"remoteFile","id":"r12","unique_id":"rich-u-896001","is_uploading_active":false,"is_uploading_completed":true,"uploaded_size":1000}},"width":800,"height":600,"progressive_sizes":[]}]},"caption":{"@type":"pageBlockCaption","text":{"@type":"richTextPlain","text":"The team"},"credit":{"@type":"richTextPlain","text":""}},"url":"","has_spoiler":false},{"@type":"pageBlockList","items":[{"@type":"pageBlockListItem","label":"1.","blocks":[{"@type":"pageBlockVideo","video":{"@type":"video","duration":12,"width":640,"height":360,"file_name":"clip.mp4","mime_type":"video/mp4","has_stickers":false,"supports_streaming":true,"video":{"@type":"file","id":13,"size":1000,"expected_size":1000,"remote":{"@type":"remoteFile","id":"r13","unique_id":"rich-u-896002","is_uploading_active":false,"is_uploading_completed":true,"uploaded_size":1000}}},"caption":{"@type":"pageBlockCaption","text":{"@type":"richTextPlain","text":""},"credit":{"@type":"richTextPlain","text":""}},"need_autoplay":true,"is_looped":false,"has_spoiler":false}],"has_checkbox":false,"is_checked":false,"value":1,"type":""}]},{"@type":"pageBlockTable","caption":{"@type":"richTextPlain","text":"Scores"},"cells":[[{"@type":"pageBlockTableCell","text":{"@type":"richTextPlain","text":"Name"},"is_header":true,"colspan":1,"rowspan":1,"align":{"@type":"pageBlockHorizontalAlignmentCenter"},"valign":{"@type":"pageBlockVerticalAlignmentMiddle"}}]],"is_bordered":true,"is_striped":false,"is_compact":false},{"@type":"pageBlockFancyNew"}],"is_rtl":false,"is_full":true}}}',
	 NULL, NULL, 1794927700),
	(4302, -1001000000042, 730002, 9299000001, 1794927720, 'rich', 'A long page',
	 'messageRichMessage', '{"blocks":40,"is_rtl":true,"tdlib":{"@type":"messageRichMessage"},"tdlib_omitted_bytes":300000}', NULL, NULL, 0);

INSERT INTO telegram_group_message_edits (group_message_id, content_type, text, edit_date,
	content_extra) VALUES
	(4301, 'rich', 'Release notes', 0,
	 '{"blocks":1,"is_rtl":false,"tdlib":{"@type":"messageRichMessage","message":{"@type":"richMessage","blocks":[{"@type":"pageBlockTitle","title":{"@type":"richTextPlain","text":"Release notes"}},{"@type":"pageBlockPhoto","photo":{"@type":"photo","has_stickers":false,"sizes":[{"@type":"photoSize","type":"m","photo":{"@type":"file","id":11,"size":1000,"expected_size":1000,"remote":{"@type":"remoteFile","id":"r11","unique_id":"rich-u-small","is_uploading_active":false,"is_uploading_completed":true,"uploaded_size":1000}},"width":320,"height":240,"progressive_sizes":[]},{"@type":"photoSize","type":"y","photo":{"@type":"file","id":12,"size":1000,"expected_size":1000,"remote":{"@type":"remoteFile","id":"r12","unique_id":"rich-u-896001","is_uploading_active":false,"is_uploading_completed":true,"uploaded_size":1000}},"width":800,"height":600,"progressive_sizes":[]}]},"caption":{"@type":"pageBlockCaption","text":{"@type":"richTextPlain","text":"The team"},"credit":{"@type":"richTextPlain","text":""}},"url":"","has_spoiler":false}],"is_rtl":false,"is_full":false}}}');

INSERT INTO telegram_message_files (group_message_id, role, position, file_id) VALUES
	(4301, 'rich', 0, 896001),
	(4301, 'rich', 1, 896002);
