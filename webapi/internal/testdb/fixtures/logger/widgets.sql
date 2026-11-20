-- Widget messages (docs/design/message-types.md section 4.2), for the
-- WidgetDetail the chat and the message page send. Loaded after
-- "sample", whose user is the sender. Invented; nothing here comes from
-- a real archive.
--
-- Ids, beside the sample's and the other fixtures':
--   groups     -1001000000044 (Widget Lab, not exposed)
--   messages   group rows 4501 - 4504 (Widget Lab, message ids 750001 -
--              750004)
-- By row:
--   4501  a shared widget of 320 x 240 with a caption, whose HTML holds
--         a script
--   4502  a widget whose JSON was over the daemon's cap: the class and
--         curated keys only, full width and no fixed height
--   4503  a widget stored before the curated keys: TDLib's JSON alone
--   4504  a widget logged without content_extra

INSERT INTO telegram_groups (id, type, title, description, msg_count, created_at, updated_at)
	VALUES (-1001000000044, 'supergroup', 'Widget Lab', 'Widgets.', 4,
	 '2025-06-06 06:06:00', '2026-11-17 15:00:00');

INSERT INTO telegram_group_messages (id, chat_id, message_id, sender_user_id, date,
	content_type, text, tdlib_type, content_extra, service_type, file_id, edit_date) VALUES
	(4501, -1001000000044, 750001, 9299000001, 1794927660, 'widget', 'Open the board',
	 'messageWidget', '{"width":320,"height":240,"has_caption":true,"widget_id":"9007199254740993","tdlib":{"@type":"messageWidget","width":320,"height":240,"html":"<script>alert(1)</script><b>Scores</b>","widget_id":"9007199254740993","caption":{"@type":"formattedText","text":"Open the board","entities":[]}}}',
	 NULL, NULL, 0),
	(4502, -1001000000044, 750002, 9299000001, 1794927720, 'widget', NULL,
	 'messageWidget', '{"width":0,"height":0,"has_caption":false,"tdlib":{"@type":"messageWidget"},"tdlib_omitted_bytes":300000}',
	 NULL, NULL, 0),
	(4503, -1001000000044, 750003, 9299000001, 1794927780, 'widget', NULL,
	 'messageWidget', '{"tdlib":{"@type":"messageWidget","width":120,"height":80,"html":"<p>old</p>","widget_id":"42","caption":{"@type":"formattedText","text":"","entities":[]}}}',
	 NULL, NULL, 0),
	(4504, -1001000000044, 750004, 9299000001, 1794927840, 'widget', NULL,
	 'messageWidget', NULL, NULL, NULL, 0);
