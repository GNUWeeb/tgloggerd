-- Drops the preferences, pins, saved searches and session records.
-- Roll both web services back first: they read these tables on every
-- signed-in request.
DROP TABLE web_sessions;
DROP TABLE web_saved_searches;
DROP TABLE web_pinned_groups;
DROP TABLE web_user_settings;
