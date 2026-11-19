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
// title a group was created with, and whether a background was set
// for its setter alone. It is read from content_extra, and for the
// photo, the pin and the topic from the rows they live in, in one
// batch per page (loadServiceDetail). A chat it names goes through the
// request scope as the refs do: one the viewer may not see, or the
// archive has no row for, is left out.

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
}

// community is the community's chat id; 0 when the id is not one. A
// community's id is not a chat's in TDLib; the group it stands for can
// only be found where it is stored under a chat id, which is negative.
func (x *serviceDetailExtra) community() int64 {
	if x.CommunityID < 0 {
		return x.CommunityID
	}
	return 0
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

// isTopicType is whether a service type is about a forum topic.
func isTopicType(t string) bool {
	return strings.HasPrefix(t, "forum_topic_")
}

// addServiceDetail records the detail of a service message of the
// page, with the chats it names, the pinned message and the topic it
// is about.
func (k *kinds) addServiceDetail(m logdb.Message, chats, pins, topics *[]int64) {
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
	case t == "chat_added_to_community", t == "chat_removed_from_community",
		t == "chat_join_from_community":
		*chats = append(*chats, x.community())
	case t == "chat_upgrade_from":
		*chats = append(*chats, x.basicGroup())
	}
}

// loadServiceDetail loads the pinned messages and forum topics of the
// page's service messages, and what the pinned ones' summaries read.
func (s *Server) loadServiceDetail(ctx context.Context, k *kinds, pins, topics []int64, users, chats, polls *[]int64) error {
	var err error
	if k.pins, err = logdb.TargetsByID(ctx, s.ro, k.chat, pins); err != nil {
		return err
	}
	for _, t := range k.pins {
		if t.Snippet == "" {
			k.addSummarised(t.RowID, t.ContentType, t.Extra, users, chats, polls)
		}
	}
	k.topics, err = logdb.TopicsByID(ctx, s.ro, k.chat, topics)
	return err
}

type serviceTopicJSON struct {
	ID        *int64  `json:"id"`
	Name      *string `json:"name"`
	IconColor *int64  `json:"icon_color"`
}

// The variants of ServiceDetail, tagged by kind.
type (
	servicePhotoJSON struct {
		Kind  string         `json:"kind"`
		Photo *chatMediaJSON `json:"photo"`
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
		Kind      string      `json:"kind"`
		Community chatRefJSON `json:"community"`
	}
	serviceTitleJSON struct {
		Kind       string       `json:"kind"`
		Title      string       `json:"title"`
		BasicGroup *chatRefJSON `json:"basic_group"`
	}
	serviceBackgroundJSON struct {
		Kind        string `json:"kind"`
		OnlyForSelf bool   `json:"only_for_self"`
	}
)

// storedRef is the group id as the viewer may see it, only when the
// archive has its row: a link to a group it never logged would lead
// nowhere.
func (v view) storedRef(k kinds, id int64) *chatRefJSON {
	if _, ok := k.groups[id]; !ok || id >= 0 {
		return nil
	}
	return v.refPtr(k, id)
}

// serviceDetail is a service message's ServiceDetail, one of the
// variants above; nil when its type has none or nothing of it is
// known or visible.
func (v view) serviceDetail(k kinds, m logdb.Message) any {
	x := k.serviceDetail[m.RowID]
	if x == nil {
		return nil
	}
	switch t := m.ServiceType; {
	case isPhotoType(t):
		if f, ok := messageFile(m, "chat_photo", 0); ok {
			return servicePhotoJSON{Kind: "photo", Photo: v.media("photo", f, x.Photo)}
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
		if tj := serviceTopic(k, m, x); tj != nil {
			return serviceTopicDetailJSON{Kind: "topic", Topic: *tj}
		}
	case t == "chat_added_to_community", t == "chat_removed_from_community",
		t == "chat_join_from_community":
		if r := v.storedRef(k, x.community()); r != nil && x.community() != 0 {
			return serviceCommunityJSON{Kind: "community", Community: *r}
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
		return serviceBackgroundJSON{Kind: "background", OnlyForSelf: x.OnlyForSelf}
	}
	return nil
}

// serviceTopic is the topic a topic line is about: its id (the
// message's own forum topic), and its name and icon colour as the
// action gives them, else as the topic was last seen; nil when none of
// them is known.
func serviceTopic(k kinds, m logdb.Message, x *serviceDetailExtra) *serviceTopicJSON {
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
	if j.ID == nil && j.Name == nil && j.IconColor == nil {
		return nil
	}
	return &j
}
