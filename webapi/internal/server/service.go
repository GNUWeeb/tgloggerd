// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/logdb"
)

// A service message's action as the viewer may see it
// (webapi/openapi.yaml: ServiceInfo; docs/design/message-types.md
// section 4.2.3): its type, the users and chats it names, the message
// it is about, and its text. The logger's own description (the text
// column, describe_service_message in src/tgloggerd/MessageExtract.cpp)
// names nobody: "added 2 members to the group". For the actions whose
// detail names people, the text is rebuilt from content_extra with the
// names of those the viewer may see, and counts the others without
// naming them ("added Dewi Kusuma and 1 other member to the group").
// The stored text is sent only where nothing visible can be named,
// which is never a name the viewer may not see.

// serviceExtra is content_extra as a service message's refs read it.
type serviceExtra struct {
	UserIDs       []int64    `json:"user_ids"`
	UserID        int64      `json:"user_id"`
	NewOwner      int64      `json:"new_owner_user_id"`
	FromUser      int64      `json:"from_user_id"`
	Gifter        int64      `json:"gifter_user_id"`
	ReceiverUser  int64      `json:"receiver_user_id"`
	Bot           int64      `json:"bot_user_id"`
	ManagerBot    int64      `json:"manager_bot_user_id"`
	SenderUser    int64      `json:"sender_user_id"`
	Winners       []int64    `json:"winner_user_ids"`
	Users         []partyRef `json:"users"`
	Sender        *partyRef  `json:"sender"`
	Receiver      *partyRef  `json:"receiver"`
	Owner         *partyRef  `json:"owner"`
	Creator       *partyRef  `json:"creator"`
	Traveler      *partyRef  `json:"traveler"`
	Watcher       *partyRef  `json:"watcher"`
	BoostedChat   int64      `json:"boosted_chat_id"`
	InvoiceChat   int64      `json:"invoice_chat_id"`
	Chat          *partyRef  `json:"chat"`
	MessageID     int64      `json:"message_id"`
	PollMsg       int64      `json:"poll_message_id"`
	ChecklistMsg  int64      `json:"checklist_message_id"`
	GiveawayMsg   int64      `json:"giveaway_message_id"`
	GameMsg       int64      `json:"game_message_id"`
	RequestMsg    int64      `json:"request_message_id"`
	SuggestedMsg  int64      `json:"suggested_post_message_id"`
	OfferMsg      int64      `json:"offer_message_id"`
	GiftMsg       int64      `json:"gift_message_id"`
	OldBackground int64      `json:"old_background_message_id"`
}

func parseServiceExtra(raw []byte) *serviceExtra {
	if len(raw) == 0 {
		return nil
	}
	var x serviceExtra
	if json.Unmarshal(raw, &x) != nil {
		return nil
	}
	return &x
}

// parties are the users and chats the action names, in the order
// refs lists them: a positive id is a user (or a private chat, which
// is its user), a negative one a group or channel.
func (x *serviceExtra) parties() []int64 {
	var out []int64
	add := func(ids ...int64) {
		for _, id := range ids {
			if id != 0 {
				out = append(out, id)
			}
		}
	}
	add(x.UserIDs...)
	add(x.UserID, x.NewOwner, x.FromUser, x.Gifter, x.ReceiverUser, x.Bot, x.ManagerBot,
		x.SenderUser)
	add(x.Winners...)
	for _, u := range x.Users {
		add(u.id())
	}
	for _, p := range []*partyRef{x.Sender, x.Receiver, x.Owner, x.Creator, x.Traveler,
		x.Watcher, x.Chat} {
		if p != nil {
			add(p.id())
		}
	}
	add(x.BoostedChat, x.InvoiceChat)
	return out
}

// messageID is the message of the chat the action is about.
func (x *serviceExtra) messageID() int64 {
	for _, id := range []int64{x.MessageID, x.PollMsg, x.ChecklistMsg, x.GiveawayMsg,
		x.GameMsg, x.RequestMsg, x.SuggestedMsg, x.OfferMsg, x.GiftMsg, x.OldBackground} {
		if id != 0 {
			return id
		}
	}
	return 0
}

type serviceRefsJSON struct {
	Users     []chatRefJSON `json:"users"`
	Chats     []chatRefJSON `json:"chats"`
	MessageID *int64        `json:"message_id"`
}

type serviceJSON struct {
	Type string          `json:"type"`
	Refs serviceRefsJSON `json:"refs"`
}

// nameList joins names as a sentence lists them: "A", "A and B",
// "A, B and C".
func nameList(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// others counts the ones left unnamed: "1 other member", "2 other
// members".
func others(n int, noun string) string {
	s := strconv.Itoa(n) + " other " + noun
	if n != 1 {
		s += "s"
	}
	return s
}

// named is the names of the visible ones among ids, and how many are
// left out.
func (v view) named(k kinds, ids []int64) (names []string, hidden int) {
	for _, id := range ids {
		if r, ok := v.ref(k, id); ok {
			names = append(names, r.Title)
		} else {
			hidden++
		}
	}
	return names, hidden
}

// people is the visible ones among ids and a count of the others, as a
// list: "Dewi Kusuma and 2 other members"; "" when none is visible.
func (v view) people(k kinds, ids []int64, noun string) string {
	names, hidden := v.named(k, ids)
	if len(names) == 0 {
		return ""
	}
	if hidden > 0 {
		names = append(names, others(hidden, noun))
	}
	return nameList(names)
}

// serviceText is the action's description for the viewer: rebuilt to
// name whom it may see where the action names people, else the stored
// one, which names nobody.
func (v view) serviceText(k kinds, m logdb.Message, x *serviceExtra) string {
	if x == nil {
		return m.Text
	}
	one := func(id int64) string {
		if r, ok := v.ref(k, id); ok && id != 0 {
			return r.Title
		}
		return ""
	}
	switch m.ServiceType {
	case "chat_add_members":
		// One member who added itself joined; the sender says who.
		if len(x.UserIDs) == 1 && x.UserIDs[0] == m.SenderUser {
			return m.Text
		}
		if p := v.people(k, x.UserIDs, "member"); p != "" {
			return "added " + p + " to the group"
		}
	case "chat_delete_member":
		if x.UserID != m.SenderUser {
			if n := one(x.UserID); n != "" {
				return "removed " + n + " from the group"
			}
		}
	case "chat_owner_left":
		if n := one(x.NewOwner); n != "" {
			return "left the group and handed ownership to " + n
		}
	case "chat_owner_changed":
		if n := one(x.NewOwner); n != "" {
			return "transferred ownership of the group to " + n
		}
	case "invite_video_chat_participants":
		if p := v.people(k, x.UserIDs, "participant"); p != "" {
			return "invited " + p + " to the video chat"
		}
	case "giveaway_winners":
		chat, winners := one(x.BoostedChat), v.people(k, x.Winners, "winner")
		switch {
		case chat != "" && winners != "":
			return "announced the winners of the giveaway in " + chat + ": " + winners
		case chat != "":
			return "announced the winners of the giveaway in " + chat
		case winners != "":
			return "announced the giveaway winners: " + winners
		}
	case "chat_shared":
		if x.Chat != nil {
			if n := one(x.Chat.id()); n != "" {
				return "shared " + n + " with the bot"
			}
		}
	}
	return m.Text
}

// service is a service message's ServiceInfo, nil for other kinds.
func (v view) service(k kinds, m logdb.Message) *serviceJSON {
	if m.ContentType != "service" {
		return nil
	}
	j := &serviceJSON{Type: m.ServiceType,
		Refs: serviceRefsJSON{Users: []chatRefJSON{}, Chats: []chatRefJSON{}}}
	x := k.service[m.RowID]
	if x == nil {
		return j
	}
	seen := map[int64]bool{}
	for _, id := range x.parties() {
		if seen[id] {
			continue
		}
		seen[id] = true
		r, ok := v.ref(k, id)
		switch {
		case !ok:
		case r.Kind == "user":
			j.Refs.Users = append(j.Refs.Users, r)
		default:
			j.Refs.Chats = append(j.Refs.Chats, r)
		}
	}
	if id := x.messageID(); id != 0 {
		j.Refs.MessageID = &id
	}
	return j
}
