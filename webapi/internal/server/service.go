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
// which is never a name the viewer may not see. The rebuilt text also
// goes out piece by piece (ServiceInfo.parts), each name pointing at
// its ref, so a client can link it without matching names in the text:
// two people can share a name, and a name can contain the words around
// it.

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

// servicePartJSON is one piece of a rebuilt text (ServicePart): plain
// text, or a name, which points at its user or chat in refs by index.
type servicePartJSON struct {
	Text string `json:"text"`
	User *int   `json:"user,omitempty"`
	Chat *int   `json:"chat,omitempty"`
}

type serviceJSON struct {
	Type string          `json:"type"`
	Refs serviceRefsJSON `json:"refs"`
	// Parts is the rebuilt text piece by piece, so a client can link
	// the names; absent where the stored text, which names nobody, is
	// sent.
	Parts []servicePartJSON `json:"parts,omitempty"`
}

// servicePart is a piece of a rebuilt service text: plain text, or
// the name of the user or chat id (non-zero), one refs lists.
type servicePart struct {
	text string
	id   int64
}

// plain is text that names nobody, as a piece of a line.
func plain(s string) []servicePart {
	return []servicePart{{text: s}}
}

// line joins pieces into a service text, merging neighbouring plain
// text, so the parts read as the text does.
func line(pieces ...[]servicePart) []servicePart {
	var out []servicePart
	for _, ps := range pieces {
		for _, p := range ps {
			if n := len(out); n > 0 && p.id == 0 && out[n-1].id == 0 {
				out[n-1].text += p.text
				continue
			}
			out = append(out, p)
		}
	}
	return out
}

// partsText is the text the parts spell.
func partsText(ps []servicePart) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.text)
	}
	return b.String()
}

// nameList joins names as a sentence lists them: "A", "A and B",
// "A, B and C".
func nameList(names []servicePart) []servicePart {
	var out []servicePart
	for i, n := range names {
		switch {
		case i == 0:
		case i == len(names)-1:
			out = append(out, servicePart{text: " and "})
		default:
			out = append(out, servicePart{text: ", "})
		}
		out = append(out, n)
	}
	return out
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
func (v view) named(k kinds, ids []int64) (names []servicePart, hidden int) {
	for _, id := range ids {
		if r, ok := v.ref(k, id); ok && id != 0 {
			names = append(names, servicePart{text: r.Title, id: id})
		} else {
			hidden++
		}
	}
	return names, hidden
}

// people is the visible ones among ids and a count of the others, as a
// list: "Dewi Kusuma and 2 other members"; nil when none is visible.
func (v view) people(k kinds, ids []int64, noun string) []servicePart {
	names, hidden := v.named(k, ids)
	if len(names) == 0 {
		return nil
	}
	if hidden > 0 {
		names = append(names, servicePart{text: others(hidden, noun)})
	}
	return nameList(names)
}

// serviceText is the action's description for the viewer
// (serviceParts), as text.
func (v view) serviceText(k kinds, m logdb.Message, x *serviceExtra) string {
	if ps := v.serviceParts(k, m, x); ps != nil {
		return partsText(ps)
	}
	return m.Text
}

// serviceParts is the action's description for the viewer, rebuilt to
// name whom it may see where the action names people; nil where the
// stored text, which names nobody, stands. Text and parts both come
// from here, so they always agree.
func (v view) serviceParts(k kinds, m logdb.Message, x *serviceExtra) []servicePart {
	if x == nil {
		return nil
	}
	one := func(id int64) []servicePart {
		if r, ok := v.ref(k, id); ok && id != 0 {
			return []servicePart{{text: r.Title, id: id}}
		}
		return nil
	}
	switch m.ServiceType {
	case "chat_add_members":
		// One member who added itself joined; the sender says who.
		if len(x.UserIDs) == 1 && x.UserIDs[0] == m.SenderUser {
			return nil
		}
		if p := v.people(k, x.UserIDs, "member"); p != nil {
			return line(plain("added "), p, plain(" to the group"))
		}
	case "chat_delete_member":
		if x.UserID != m.SenderUser {
			if n := one(x.UserID); n != nil {
				return line(plain("removed "), n, plain(" from the group"))
			}
		}
	case "chat_owner_left":
		if n := one(x.NewOwner); n != nil {
			return line(plain("left the group and handed ownership to "), n)
		}
	case "chat_owner_changed":
		if n := one(x.NewOwner); n != nil {
			return line(plain("transferred ownership of the group to "), n)
		}
	case "invite_video_chat_participants":
		if p := v.people(k, x.UserIDs, "participant"); p != nil {
			return line(plain("invited "), p, plain(" to the video chat"))
		}
	case "giveaway_winners":
		chat, winners := one(x.BoostedChat), v.people(k, x.Winners, "winner")
		switch {
		case chat != nil && winners != nil:
			return line(plain("announced the winners of the giveaway in "), chat,
				plain(": "), winners)
		case chat != nil:
			return line(plain("announced the winners of the giveaway in "), chat)
		case winners != nil:
			return line(plain("announced the giveaway winners: "), winners)
		}
	case "chat_shared":
		if x.Chat != nil {
			if n := one(x.Chat.id()); n != nil {
				return line(plain("shared "), n, plain(" with the bot"))
			}
		}
	}
	return nil
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
	j.Parts = serviceRefParts(j.Refs, v.serviceParts(k, m, x))
	return j
}

// serviceRefParts is the parts as ServicePart writes them, each name
// pointing at its user or chat in refs. A name always has its ref, as
// both name only the visible ones of parties; one without would stay
// plain text.
func serviceRefParts(refs serviceRefsJSON, ps []servicePart) []servicePartJSON {
	if ps == nil {
		return nil
	}
	index := func(rs []chatRefJSON, id int64) *int {
		for i, r := range rs {
			if r.ID == id {
				return &i
			}
		}
		return nil
	}
	out := make([]servicePartJSON, 0, len(ps))
	for _, p := range ps {
		j := servicePartJSON{Text: p.text}
		if p.id != 0 {
			if j.User = index(refs.Users, p.id); j.User == nil {
				j.Chat = index(refs.Chats, p.id)
			}
		}
		out = append(out, j)
	}
	return out
}
