// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"strings"

	"tgloggerd/webapi/internal/logdb"
)

// A service action's detail beyond its line (webapi/openapi.yaml:
// ServiceDetail; docs/design/message-types.md section 4.2.3): the new
// group photo, a video chat's length or start, the pinned message's
// snippet, a poll option's text, the forum topic, the community, the
// title a group was created with, and the background set, with whether
// it was set for its setter alone. It is read from content_extra, and
// for the photo (and an animated one's video), the background's image,
// the pin, the topic and the community from the rows they live in, in
// one batch per page (loadServiceDetail). A chat it names goes through
// the request scope as the refs do: one the viewer may not see, or the
// archive has no row for, is left out; a community is shown to a
// viewer who may see one of its chats.

// serviceDetailExtra is content_extra as a service action's detail
// reads it.
type serviceDetailExtra struct {
	// Photo is the new photo's own detail (width, height,
	// minithumbnail), the media detail of its file.
	Photo     json.RawMessage `json:"photo"`
	Duration  int64           `json:"duration"`
	StartDate int64           `json:"start_date"`
	MessageID int64           `json:"message_id"`
	Option    *struct {
		Text string `json:"text"`
	} `json:"option"`
	Name         string `json:"name"`
	IconColor    *int64 `json:"icon_color"`
	CommunityID  int64  `json:"community_id"`
	Title        string `json:"title"`
	BasicGroupID int64  `json:"basic_group_id"`
	OnlyForSelf  bool   `json:"only_for_self"`
	// IconEmoji is the custom emoji icon a topic created or edited
	// sets (a decimal string; absent for none); IconEdited says an
	// edit set it, or removed it.
	IconEmoji  looseID `json:"icon_custom_emoji_id"`
	IconEdited bool    `json:"icon_edited"`
	// Background is the background a chat_set_background set; nil
	// on rows logged before it was kept.
	Background *backgroundExtra `json:"background"`
}

// backgroundExtra is a background as content_extra keeps it
// (docs/design/message-types.md section 4.2.3): the flags are absent
// when false and intensity when 0.
type backgroundExtra struct {
	Type       string          `json:"type"`
	Dark       bool            `json:"dark"`
	Fill       *backgroundFill `json:"fill"`
	Intensity  int64           `json:"intensity"`
	IsInverted bool            `json:"is_inverted"`
	IsMoving   bool            `json:"is_moving"`
	IsBlurred  bool            `json:"is_blurred"`
	ThemeName  string          `json:"theme_name"`
}

// basicGroup is the chat id of the basic group a supergroup was
// upgraded from (TDLib's chat id of basic group n is -n).
func (x *serviceDetailExtra) basicGroup() int64 {
	if x.BasicGroupID > 0 {
		return -x.BasicGroupID
	}
	return 0
}

// isPhotoType is whether a service type shows a photo (role
// chat_photo).
func isPhotoType(t string) bool {
	return t == "chat_change_photo" || t == "suggest_profile_photo"
}

// isCommunityType is whether a service type names a community.
// chat_removed_from_community is one too, though TDLib gives it no
// community id.
func isCommunityType(t string) bool {
	return t == "chat_added_to_community" || t == "chat_removed_from_community" ||
		t == "chat_join_from_community"
}

// isTopicType is whether a service type is about a forum topic.
func isTopicType(t string) bool {
	return strings.HasPrefix(t, "forum_topic_")
}

// addServiceDetail records the detail of a service message of the
// page, with the chats it names, the pinned message, the topic and the
// community it is about.
func (k *kinds) addServiceDetail(m logdb.Message, chats, pins, topics, communities *[]int64) {
	var x serviceDetailExtra
	if len(m.Extra) == 0 || json.Unmarshal(m.Extra, &x) != nil {
		// The photo is a file of the message, which a row without
		// its detail may have all the same; the rest is the detail.
		if !isPhotoType(m.ServiceType) {
			return
		}
		x = serviceDetailExtra{}
	}
	if k.serviceDetail == nil {
		k.serviceDetail = map[uint64]*serviceDetailExtra{}
	}
	k.serviceDetail[m.RowID] = &x
	switch t := m.ServiceType; {
	case t == "pin_message":
		*pins = append(*pins, x.MessageID)
	case isTopicType(t):
		*topics = append(*topics, m.TopicID)
	case isCommunityType(t):
		*communities = append(*communities, x.CommunityID)
	case t == "chat_upgrade_from":
		*chats = append(*chats, x.basicGroup())
	}
}

// loadServiceDetail loads the pinned messages, forum topics and
// communities of the page's service messages, and what the pinned
// ones' summaries read.
func (s *Server) loadServiceDetail(ctx context.Context, k *kinds, pins, topics, communities []int64,
	users, chats, polls *[]int64) error {
	var err error
	if k.pins, err = logdb.TargetsByID(ctx, s.ro, k.chat, pins); err != nil {
		return err
	}
	for _, t := range k.pins {
		if t.Snippet == "" {
			k.addSummarised(t.RowID, t.ContentType, t.Extra, users, chats, polls)
		}
	}
	if k.topics, err = logdb.TopicsByID(ctx, s.ro, k.chat, topics); err != nil {
		return err
	}
	k.communities, err = logdb.CommunitiesByID(ctx, s.ro, communities)
	return err
}

type communityJSON struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Photo *string `json:"photo"`
}

type serviceTopicJSON struct {
	ID          *int64           `json:"id"`
	Name        *string          `json:"name"`
	IconColor   *int64           `json:"icon_color"`
	IconEmojiID *string          `json:"icon_custom_emoji_id"`
	IconEmoji   *customEmojiJSON `json:"icon_custom_emoji"`
}

// The variants of ServiceDetail, tagged by kind.
type (
	servicePhotoJSON struct {
		Kind      string         `json:"kind"`
		Photo     *chatMediaJSON `json:"photo"`
		Animation *chatMediaJSON `json:"animation"`
	}
	serviceVideoChatJSON struct {
		Kind      string  `json:"kind"`
		Duration  *int64  `json:"duration"`
		StartDate *string `json:"start_date"`
	}
	servicePinJSON struct {
		Kind    string `json:"kind"`
		Snippet string `json:"snippet"`
	}
	servicePollOptionJSON struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}
	serviceTopicDetailJSON struct {
		Kind  string           `json:"kind"`
		Topic serviceTopicJSON `json:"topic"`
	}
	serviceCommunityJSON struct {
		Kind      string        `json:"kind"`
		Community communityJSON `json:"community"`
	}
	serviceTitleJSON struct {
		Kind       string       `json:"kind"`
		Title      string       `json:"title"`
		BasicGroup *chatRefJSON `json:"basic_group"`
	}
	serviceBackgroundJSON struct {
		Kind        string          `json:"kind"`
		OnlyForSelf bool            `json:"only_for_self"`
		Type        *string         `json:"type"`
		Dark        bool            `json:"dark"`
		Image       *chatMediaJSON  `json:"image"`
		Fill        *backgroundFill `json:"fill"`
		Intensity   *int64          `json:"intensity"`
		IsInverted  bool            `json:"is_inverted"`
		IsMoving    bool            `json:"is_moving"`
		IsBlurred   bool            `json:"is_blurred"`
		ThemeName   *string         `json:"theme_name"`
	}
)

// backgroundFill is BackgroundFill, as content_extra keeps it too.
type backgroundFill struct {
	Type          string  `json:"type"`
	Colors        []int64 `json:"colors"`
	RotationAngle int64   `json:"rotation_angle"`
}

// storedRef is the group id as the viewer may see it, only when the
// archive has its row: a link to a group it never logged would lead
// nowhere.
func (v view) storedRef(k kinds, id int64) *chatRefJSON {
	if _, ok := k.groups[id]; !ok || id >= 0 {
		return nil
	}
	return v.refPtr(k, id)
}

// seesCommunity is whether the viewer may see a community: when it may
// see one of the chats its full info listed, or every chat (and so
// those not listed yet, as before its full info arrives).
func (v view) seesCommunity(c logdb.Community) bool {
	if v.sc.SeesAllGroups() && v.sc.SeesPrivate() {
		return true
	}
	for _, id := range c.Chats {
		if v.sc.SeesChat(id) {
			return true
		}
	}
	return false
}

// serviceDetail is a service message's ServiceDetail, one of the
// variants above; nil when its type has none or nothing of it is
// known or visible.
func (v view) serviceDetail(k kinds, m logdb.Message) any {
	switch t := m.ServiceType; {
	case isGiftType(t):
		return v.giftDetail(k, m)
	case t == "chat_set_theme":
		return v.themeDetail(k, m)
	}
	x := k.serviceDetail[m.RowID]
	if x == nil {
		return nil
	}
	switch t := m.ServiceType; {
	case isPhotoType(t):
		if f, ok := messageFile(m, "chat_photo", 0); ok {
			return servicePhotoJSON{Kind: "photo", Photo: v.media("photo", f, x.Photo),
				Animation: v.photoAnimation(m, f, x)}
		}
	case t == "video_chat_ended":
		d := x.Duration
		return serviceVideoChatJSON{Kind: "video_chat", Duration: &d}
	case t == "video_chat_scheduled":
		return serviceVideoChatJSON{Kind: "video_chat", StartDate: isoPtr(x.StartDate)}
	case t == "pin_message":
		if p, ok := k.pins[x.MessageID]; ok && x.MessageID != 0 {
			s, _ := cutRunes(v.snippetOf(k, p.Snippet, p.RowID, p.ContentType, p.FileName),
				replySnippetRunes)
			return servicePinJSON{Kind: "pin", Snippet: s}
		}
	case t == "poll_option_added", t == "poll_option_deleted":
		if x.Option != nil && x.Option.Text != "" {
			return servicePollOptionJSON{Kind: "poll_option", Text: x.Option.Text}
		}
	case isTopicType(t):
		if tj := v.serviceTopic(k, m, x); tj != nil {
			return serviceTopicDetailJSON{Kind: "topic", Topic: *tj}
		}
	case isCommunityType(t):
		if c, ok := k.communities[x.CommunityID]; ok && x.CommunityID != 0 &&
			c.Name != "" && v.seesCommunity(c) {
			return serviceCommunityJSON{Kind: "community", Community: communityJSON{
				ID: c.ID, Name: c.Name, Photo: v.photo(c.Photo)}}
		}
	case t == "chat_create", t == "chat_upgrade_from":
		j := serviceTitleJSON{Kind: "title", Title: x.Title}
		if id := x.basicGroup(); id != 0 && t == "chat_upgrade_from" {
			j.BasicGroup = v.storedRef(k, id)
		}
		if j.Title != "" || j.BasicGroup != nil {
			return j
		}
	case t == "chat_set_background":
		return v.serviceBackground(m, x)
	}
	return nil
}

// photoAnimation is an animated photo's video, the chat_photo file at
// position 1, as an animation the size of the photo, its poster the
// still photo when that is stored; nil for a still photo.
func (v view) photoAnimation(m logdb.Message, still logdb.FileRef, x *serviceDetailExtra) *chatMediaJSON {
	f, ok := messageFile(m, "chat_photo", 1)
	if !ok {
		return nil
	}
	md := v.media("animation", f, x.Photo)
	if md != nil && storedFile(still) {
		u := v.photo(still)
		md.Poster, md.Thumb = u, u
	}
	return md
}

// backgroundTypes and fillTypes are the values of
// ServiceBackgroundDetail.type and BackgroundFill.type; one the daemon
// may store later, which this service does not know yet, is left out.
var (
	backgroundTypes = map[string]bool{"wallpaper": true, "pattern": true, "fill": true,
		"chat_theme": true}
	fillTypes = map[string]bool{"solid": true, "gradient": true, "freeform_gradient": true}
)

// backgroundImageExts are the background files a browser draws as they
// are: a wallpaper's JPEG and a pattern's PNG. A pattern in Telegram's
// own format (gzipped SVG) goes out as a file.
var backgroundImageExts = map[string]bool{"jpg": true, "jpeg": true, "png": true}

// serviceBackground is a chat_set_background's detail: the background
// as content_extra keeps it, with its image, the message's file of
// role background.
func (v view) serviceBackground(m logdb.Message, x *serviceDetailExtra) serviceBackgroundJSON {
	j := serviceBackgroundJSON{Kind: "background", OnlyForSelf: x.OnlyForSelf}
	if f, ok := messageFile(m, "background", 0); ok {
		ctype := "document"
		if backgroundImageExts[strings.ToLower(f.Ext)] {
			ctype = "photo"
		}
		j.Image = v.media(ctype, f, nil)
	}
	b := x.Background
	if b == nil {
		return j
	}
	if backgroundTypes[b.Type] {
		t := b.Type
		j.Type = &t
	}
	j.Dark = b.Dark
	if fl := b.Fill; fl != nil && fillTypes[fl.Type] && (b.Type == "fill" || b.Type == "pattern") {
		colors := make([]int64, 0, len(fl.Colors))
		for _, c := range fl.Colors {
			colors = append(colors, c&0xffffff)
		}
		j.Fill = &backgroundFill{Type: fl.Type, Colors: colors, RotationAngle: fl.RotationAngle}
	}
	switch b.Type {
	case "pattern":
		n := b.Intensity
		j.Intensity = &n
		j.IsInverted, j.IsMoving = b.IsInverted, b.IsMoving
	case "wallpaper":
		j.IsMoving, j.IsBlurred = b.IsMoving, b.IsBlurred
	case "chat_theme":
		if b.ThemeName != "" {
			n := b.ThemeName
			j.ThemeName = &n
		}
	}
	return j
}

// serviceTopicIcon is the custom emoji of the icon of the topic a
// topic line is about: the action's own when it sets one (or removes
// it: 0), else the topic's as last seen; 0 for none.
func serviceTopicIcon(k kinds, m logdb.Message, x *serviceDetailExtra) int64 {
	if x.IconColor != nil || x.IconEdited {
		return int64(x.IconEmoji)
	}
	if t, ok := k.topics[m.TopicID]; ok && m.TopicID != 0 {
		return t.IconEmoji
	}
	return 0
}

// serviceTopic is the topic a topic line is about: its id (the
// message's own forum topic), and its name, icon colour and icon custom
// emoji as the action gives them, else as the topic was last seen; nil
// when none of them is known.
func (v view) serviceTopic(k kinds, m logdb.Message, x *serviceDetailExtra) *serviceTopicJSON {
	var j serviceTopicJSON
	if m.TopicID != 0 {
		id := m.TopicID
		j.ID = &id
	}
	if t, ok := k.topics[m.TopicID]; ok && m.TopicID != 0 {
		name, color := t.Name, t.Color
		j.Name, j.IconColor = &name, &color
	}
	if x.Name != "" {
		name := x.Name
		j.Name = &name
	}
	if x.IconColor != nil {
		color := *x.IconColor
		j.IconColor = &color
	}
	if id := serviceTopicIcon(k, m, x); id != 0 {
		j.IconEmojiID, j.IconEmoji = idString(id), v.customEmojiOf(k, id)
	}
	if j.ID == nil && j.Name == nil && j.IconColor == nil && j.IconEmojiID == nil {
		return nil
	}
	return &j
}
