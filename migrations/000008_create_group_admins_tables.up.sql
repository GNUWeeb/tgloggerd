-- Group administrators and their privileges, plus a change history.
-- Telegram does not push admin changes to a regular user account
-- (updateChatMember is bots-only), so the admin list is fetched on first
-- sight of a group and refreshed by periodic polling.

CREATE TABLE group_admins (
	-- Surrogate primary key.
	id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

	-- The group and the administrator user.
	group_id        BIGINT          NOT NULL COMMENT 'FK to groups.id (the chat_id).',
	user_id         BIGINT          NOT NULL COMMENT 'FK to users.id of the administrator.',

	-- Kind of admin and display attributes.
	status          ENUM('creator', 'administrator')
	                                NOT NULL COMMENT 'Owner (creator) or a regular administrator.',
	custom_title    VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'Custom title (td_api chatMember.tag_).',
	inviter_user_id BIGINT          NOT NULL DEFAULT 0 COMMENT 'User who promoted this admin; 0 if unknown. No FK.',
	joined_date     BIGINT          NOT NULL DEFAULT 0 COMMENT 'Unix timestamp of promotion/join (joined_chat_date).',

	-- Privileges (td_api::chatAdministratorRights). For a creator these are
	-- all synthesized to 1, since the owner implicitly holds every right.
	can_manage_chat          TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Access event log, boosts, hidden members, etc.',
	can_change_info          TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Change chat title, photo and settings.',
	can_post_messages        TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Create channel posts; channels only.',
	can_edit_messages        TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Edit others'' messages; channels only.',
	can_delete_messages      TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Delete messages of other users.',
	can_invite_users         TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Invite new users to the chat.',
	can_restrict_members     TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Restrict, ban or unban members.',
	can_pin_messages         TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Pin messages; basic groups & supergroups.',
	can_manage_topics        TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Manage forum topics; forum supergroups.',
	can_promote_members      TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Add/demote administrators.',
	can_manage_video_chats   TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Manage video chats.',
	can_post_stories         TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Post stories; supergroups & channels.',
	can_edit_stories         TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Edit others'' stories; supergroups & channels.',
	can_delete_stories       TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Delete others'' stories; supergroups & channels.',
	can_manage_direct_messages TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Answer channel direct messages; channels only.',
	can_manage_tags          TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Change tags of other users.',
	is_anonymous             TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Admin is hidden from the member list.',

	-- Bookkeeping.
	created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Row creation time.',
	updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP
	                                ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last time the row was updated.',

	PRIMARY KEY (id),
	UNIQUE KEY uq_group_admins (group_id, user_id),
	KEY idx_group_admins_user_id (user_id),
	CONSTRAINT fk_group_admins_group
		FOREIGN KEY (group_id) REFERENCES `groups` (id)
		ON DELETE CASCADE ON UPDATE CASCADE,
	CONSTRAINT fk_group_admins_user
		FOREIGN KEY (user_id) REFERENCES users (id)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='Current administrators of each group and their privileges.';

-- Append-only log of admin changes. An event is recorded when an admin is
-- added, removed, or their status/title/privileges change. added/updated
-- snapshot the new state; removed snapshots the state before removal.

CREATE TABLE group_admin_hist (
	id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

	group_id        BIGINT          NOT NULL COMMENT 'FK to groups.id.',
	-- No FK on user_id: the history must survive the user leaving, mirroring
	-- the other group_hist_* tables.
	user_id         BIGINT          NOT NULL COMMENT 'The administrator user.',

	action          ENUM('added', 'removed', 'updated')
	                                NOT NULL COMMENT 'What happened to the admin.',

	-- Snapshot of the admin state at the time of the event.
	status          ENUM('creator', 'administrator') NOT NULL COMMENT 'Admin kind at the event.',
	custom_title    VARCHAR(255)    NOT NULL DEFAULT '' COMMENT 'Custom title at the event.',
	can_manage_chat          TINYINT(1) NOT NULL DEFAULT 0,
	can_change_info          TINYINT(1) NOT NULL DEFAULT 0,
	can_post_messages        TINYINT(1) NOT NULL DEFAULT 0,
	can_edit_messages        TINYINT(1) NOT NULL DEFAULT 0,
	can_delete_messages      TINYINT(1) NOT NULL DEFAULT 0,
	can_invite_users         TINYINT(1) NOT NULL DEFAULT 0,
	can_restrict_members     TINYINT(1) NOT NULL DEFAULT 0,
	can_pin_messages         TINYINT(1) NOT NULL DEFAULT 0,
	can_manage_topics        TINYINT(1) NOT NULL DEFAULT 0,
	can_promote_members      TINYINT(1) NOT NULL DEFAULT 0,
	can_manage_video_chats   TINYINT(1) NOT NULL DEFAULT 0,
	can_post_stories         TINYINT(1) NOT NULL DEFAULT 0,
	can_edit_stories         TINYINT(1) NOT NULL DEFAULT 0,
	can_delete_stories       TINYINT(1) NOT NULL DEFAULT 0,
	can_manage_direct_messages TINYINT(1) NOT NULL DEFAULT 0,
	can_manage_tags          TINYINT(1) NOT NULL DEFAULT 0,
	is_anonymous             TINYINT(1) NOT NULL DEFAULT 0,

	created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'When the event was recorded.',

	PRIMARY KEY (id),
	KEY idx_group_admin_hist_group_id (group_id),
	KEY idx_group_admin_hist_user_id (user_id),
	CONSTRAINT fk_group_admin_hist_group
		FOREIGN KEY (group_id) REFERENCES `groups` (id)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='History of group administrator additions, removals and privilege changes.';
