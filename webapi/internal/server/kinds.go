// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"strconv"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The detail of the message kinds beyond text and media
// (webapi/openapi.yaml: MessageDetail; docs/design/message-types.md
// section 4.2.2): built from content_extra, the poll tables and
// telegram_message_files, for rows logged since daemon migration 000034.
// Older rows have no content_extra and get no detail, so they render
// as before. Every user and chat the detail names is resolved through
// the request scope (logdb.Refs), in one batch per page of messages:
// one the viewer may not see is left out, never named.

// listCap is how many items a long list of a detail holds (poll
// options, checklist tasks, giveaway chats, group call participants);
// its total counts them all.
const listCap = 20

// partyRef is a user or a chat as content_extra names one: {user_id}
// or {chat_id}.
type partyRef struct {
	UserID int64 `json:"user_id"`
	ChatID int64 `json:"chat_id"`
}

// id is the party by the logger's key: a user's id, a chat's id (a
// private chat's id is its user's).
func (p partyRef) id() int64 {
	if p.UserID != 0 {
		return p.UserID
	}
	return p.ChatID
}

// kindExtra is content_extra as the kinds' detail reads it. Keys that
// do not apply are zero.
type kindExtra struct {
	// location, venue
	Lat             *float64 `json:"lat"`
	Lon             *float64 `json:"lon"`
	Accuracy        float64  `json:"accuracy"`
	Live            bool     `json:"live"`
	LivePeriod      int64    `json:"live_period"`
	Heading         int64    `json:"heading"`
	ProximityRadius int64    `json:"proximity_radius"`
	Title           string   `json:"title"`
	Address         string   `json:"address"`
	Provider        string   `json:"provider"`
	VenueID         string   `json:"venue_id"`
	VenueType       string   `json:"venue_type"`
	// contact
	Phone     string `json:"phone"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Vcard     string `json:"vcard"`
	UserID    int64  `json:"user_id"`
	// dice
	Emoji string `json:"emoji"`
	Value int64  `json:"value"`
	Stake bool   `json:"stake"`
	// game, invoice: a game's description is a string, an invoice's
	// a formatted text {text, entities}.
	ShortName   string          `json:"short_name"`
	Description json.RawMessage `json:"description"`
	// poll
	PollID string `json:"poll_id"`
	// checklist
	Tasks []struct {
		ID   int64  `json:"id"`
		Text string `json:"text"`
		// Entities are kept raw: a malformed value must not
		// lose the whole detail.
		Entities       json.RawMessage `json:"entities"`
		CompletedBy    *partyRef       `json:"completed_by"`
		CompletionDate int64           `json:"completion_date"`
	} `json:"tasks"`
	OthersCanAdd  bool `json:"others_can_add_tasks"`
	OthersCanMark bool `json:"others_can_mark_tasks_as_done"`
	// story
	PosterChatID int64 `json:"poster_chat_id"`
	StoryID      int64 `json:"story_id"`
	// invoice
	Currency    string `json:"currency"`
	TotalAmount int64  `json:"total_amount"`
	IsTest      bool   `json:"is_test"`
	// giveaway
	BoostedChatID        int64   `json:"boosted_chat_id"`
	AdditionalChatIDs    []int64 `json:"additional_chat_ids"`
	WinnersSelectionDate int64   `json:"winners_selection_date"`
	OnlyNewMembers       bool    `json:"only_new_members"`
	PrizeDescription     string  `json:"prize_description"`
	WinnerCount          int64   `json:"winner_count"`
	Prize                *struct {
		PremiumMonths *int64 `json:"premium_months"`
		Stars         *int64 `json:"stars"`
	} `json:"prize"`
	// call
	Video         bool       `json:"video"`
	Group         bool       `json:"group"`
	Active        bool       `json:"active"`
	Missed        bool       `json:"missed"`
	Duration      int64      `json:"duration"`
	DiscardReason string     `json:"discard_reason"`
	Participants  []partyRef `json:"participants"`
	// paid media
	StarCount int64 `json:"star_count"`
	Items     []struct {
		Kind string `json:"kind"`
	} `json:"items"`
}

// detailKinds are the content types that have a detail.
var detailKinds = map[string]bool{"location": true, "venue": true, "contact": true,
	"dice": true, "game": true, "poll": true, "checklist": true, "story": true,
	"invoice": true, "giveaway": true, "call": true, "paid_media": true, "rich": true,
	"widget": true, "expired": true, "unsupported": true}

// parseKindExtra reads a row's content_extra for its detail; nil when
// the row has none (logged before migration 000034) or it is not JSON.
func parseKindExtra(raw []byte) *kindExtra {
	if len(raw) == 0 {
		return nil
	}
	var x kindExtra
	if json.Unmarshal(raw, &x) != nil {
		return nil
	}
	return &x
}

// pollID is the poll a poll message shows; 0 when none or malformed.
func (x *kindExtra) pollID() int64 {
	id, _ := strconv.ParseInt(x.PollID, 10, 64)
	return id
}

// giveawayChats are a giveaway's chats, the boosted one first.
func (x *kindExtra) giveawayChats() []int64 {
	var out []int64
	if x.BoostedChatID != 0 {
		out = append(out, x.BoostedChatID)
	}
	return append(out, x.AdditionalChatIDs...)
}

// parties are the users (positive ids) and chats (negative ids) a
// message's detail names.
func (x *kindExtra) parties(ctype string) (users, chats []int64) {
	add := func(id int64) {
		switch {
		case id > 0:
			users = append(users, id)
		case id < 0:
			chats = append(chats, id)
		}
	}
	switch ctype {
	case "contact":
		add(x.UserID)
	case "checklist":
		for _, t := range x.Tasks {
			if t.CompletedBy != nil {
				add(t.CompletedBy.id())
			}
		}
	case "story":
		add(x.PosterChatID)
	case "giveaway":
		for _, c := range x.giveawayChats() {
			add(c)
		}
	case "call":
		for _, p := range x.Participants {
			add(p.id())
		}
	}
	return users, chats
}

// kinds is what a page of messages' detail reads besides the rows,
// loaded once per page (Server.loadKinds).
type kinds struct {
	chat  int64 // the chat being read
	extra map[uint64]*kindExtra
	// edits are the content_extra of a message's earlier versions,
	// by its row id and in the order of its Edits; nil for a version
	// without a detail.
	edits   map[uint64][]*kindExtra
	service map[uint64]*serviceExtra
	polls   map[int64]*logdb.Poll
	users   map[int64]logdb.User  // the visible ones
	groups  map[int64]logdb.Group // the visible ones, and the chat itself
	// targets are the messages of the chat that service actions are
	// about and whose detail their text quotes (servicetext.go: a
	// checklist, a giveaway), by message id.
	targets map[int64]*kindExtra
	// mentions are the people the text entities name
	// (textentities.go).
	mentions textMentions
}

// pollClosed is whether a poll is closed: stored as closed, or past its
// close date, as Telegram does not always send the update that closes
// a poll whose time ran out, and the stored flag then stays open.
func pollClosed(closed bool, closeDate, now int64) bool {
	return closed || (closeDate != 0 && closeDate <= now)
}

// loadKinds parses the content_extra of the messages that have a
// detail and loads, in one batch for all of them, the polls they show
// and the users and groups they name, and the people their text
// entities mention, as sc may see them.
func (s *Server) loadKinds(ctx context.Context, sc scope.Scope, chat int64, ms []logdb.Message) (kinds, error) {
	k := kinds{chat: chat, extra: map[uint64]*kindExtra{}, edits: map[uint64][]*kindExtra{},
		service: map[uint64]*serviceExtra{}}
	var users, chats, polls []int64
	for _, m := range ms {
		if m.ContentType == "service" {
			if x := parseServiceExtra(m.Extra); x != nil {
				k.service[m.RowID] = x
				for _, id := range x.parties() {
					if id > 0 {
						users = append(users, id)
					} else {
						chats = append(chats, id)
					}
				}
			}
			continue
		}
		// An earlier version's detail is its own content_extra, but
		// for a poll's: the poll tables keep only the latest results,
		// so a poll's versions have none.
		for i, e := range m.Edits {
			if !detailKinds[e.ContentType] || e.ContentType == "poll" {
				continue
			}
			x := parseKindExtra(e.Extra)
			if x == nil {
				continue
			}
			if k.edits[m.RowID] == nil {
				k.edits[m.RowID] = make([]*kindExtra, len(m.Edits))
			}
			k.edits[m.RowID][i] = x
			u, c := x.parties(e.ContentType)
			users, chats = append(users, u...), append(chats, c...)
		}
		if !detailKinds[m.ContentType] {
			continue
		}
		x := parseKindExtra(m.Extra)
		if x == nil {
			continue
		}
		k.extra[m.RowID] = x
		u, c := x.parties(m.ContentType)
		users, chats = append(users, u...), append(chats, c...)
		if m.ContentType == "poll" {
			polls = append(polls, x.pollID())
		}
	}
	shown := map[int64]bool{}
	for _, id := range polls {
		shown[id] = true
	}
	// A reply to a message without text quotes its summary
	// (snippets.go), which may need its poll or its story's poster.
	for _, m := range ms {
		if r := m.Reply; r != nil && r.Resolved && r.Snippet == "" {
			k.addSummarised(r.RowID, r.ContentType, r.Extra, &users, &chats, &polls)
		}
	}
	var err error
	if k.targets, err = s.loadServiceTargets(ctx, chat, ms, k.service); err != nil {
		return k, err
	}
	if k.polls, err = logdb.PollsByID(ctx, s.ro, polls); err != nil {
		return k, err
	}
	if k.users, k.groups, err = logdb.Refs(ctx, s.ro, sc, chat, users, chats); err != nil {
		return k, err
	}
	mc := mentionCollector{s: s}
	for _, m := range ms {
		if m.ContentType == "service" {
			continue
		}
		mc.add(m.Text, m.Entities, "message")
		for i, e := range m.Edits {
			mc.add(e.Text, e.Entities, "edit")
			if xs := k.edits[m.RowID]; xs != nil && e.ContentType == "checklist" {
				mc.addTasks(xs[i])
			}
		}
		if m.ContentType == "checklist" {
			mc.addTasks(k.extra[m.RowID])
		}
	}
	for id, p := range k.polls {
		if !shown[id] {
			continue
		}
		mc.add(p.Question, p.QuestionEntities, "poll question")
		mc.add(p.Explanation, p.ExplanationEntities, "poll explanation")
	}
	k.mentions, err = mc.load(ctx, sc)
	return k, err
}

// addTasks adds the people the text of a checklist's shown tasks
// mention; x may be nil.
func (c *mentionCollector) addTasks(x *kindExtra) {
	if x == nil {
		return
	}
	for i, t := range x.Tasks {
		if i == listCap {
			break
		}
		c.add(t.Text, t.Entities, "checklist task")
	}
}

// ref is the user (positive id) or chat (negative id) as the viewer
// may see it; ok is false for one it may not see. A user or chat the
// archive has no row for is named by its id, for a viewer that would
// see it.
func (v view) ref(k kinds, id int64) (chatRefJSON, bool) {
	switch {
	case id > 0:
		if u, ok := k.users[id]; ok {
			return chatRefJSON{ID: id, Title: u.Name(), Kind: "user", Photo: v.photo(u.Photo)}, true
		}
		if v.full() {
			return chatRefJSON{ID: id, Title: "#" + strconv.FormatInt(id, 10), Kind: "user"}, true
		}
	case id < 0:
		if g, ok := k.groups[id]; ok {
			return chatRefJSON{ID: id, Title: g.Title, Kind: "group", Photo: v.photo(g.Photo)}, true
		}
		if id == k.chat || v.sc.SeesChat(id) {
			return chatRefJSON{ID: id, Title: "#" + strconv.FormatInt(id, 10), Kind: "group"}, true
		}
	}
	return chatRefJSON{}, false
}

// refUsername is the active username of the user (positive id) or
// chat (negative id), from the rows loadKinds read for the visible
// ones; nil for one the viewer may not see or without a username.
func refUsername(k kinds, id int64) *string {
	var u string
	switch {
	case id > 0:
		u = k.users[id].Username
	case id < 0:
		u = k.groups[id].Username
	}
	if u == "" {
		return nil
	}
	return &u
}

// refPtr is ref as a nullable field.
func (v view) refPtr(k kinds, id int64) *chatRefJSON {
	if r, ok := v.ref(k, id); ok {
		return &r
	}
	return nil
}

// refList is the visible ones of ids, at most listCap.
func (v view) refList(k kinds, ids []int64) []chatRefJSON {
	out := []chatRefJSON{}
	for _, id := range ids {
		if len(out) == listCap {
			break
		}
		if r, ok := v.ref(k, id); ok {
			out = append(out, r)
		}
	}
	return out
}

type liveJSON struct {
	Period          int64 `json:"period"`
	Heading         int64 `json:"heading"`
	ProximityRadius int64 `json:"proximity_radius"`
	Updates         int   `json:"updates"`
}

type locationJSON struct {
	Kind     string    `json:"kind"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	Accuracy *float64  `json:"accuracy"`
	Live     *liveJSON `json:"live"`
}

type venueJSON struct {
	Kind      string  `json:"kind"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Title     string  `json:"title"`
	Address   string  `json:"address"`
	Provider  string  `json:"provider"`
	VenueID   string  `json:"venue_id"`
	VenueType string  `json:"venue_type"`
}

type contactJSON struct {
	Kind      string       `json:"kind"`
	FirstName string       `json:"first_name"`
	LastName  string       `json:"last_name"`
	Phone     *string      `json:"phone"`
	Vcard     *string      `json:"vcard"`
	User      *chatRefJSON `json:"user"`
}

type diceJSON struct {
	Kind  string `json:"kind"`
	Emoji string `json:"emoji"`
	Value int64  `json:"value"`
	Stake bool   `json:"stake"`
}

type gameJSON struct {
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ShortName   string         `json:"short_name"`
	Photo       *chatMediaJSON `json:"photo"`
}

type pollOptionJSON struct {
	Text    string `json:"text"`
	Voters  int64  `json:"voters"`
	Percent int64  `json:"percent"`
	Correct bool   `json:"correct"`
}

type pollJSON struct {
	Kind             string           `json:"kind"`
	Question         string           `json:"question"`
	QuestionEntities []textEntityJSON `json:"question_entities"`
	Type             string           `json:"type"`
	Anonymous        bool             `json:"anonymous"`
	Multiple         bool             `json:"multiple"`
	Closed           bool             `json:"closed"`
	TotalVoters      int64            `json:"total_voters"`
	CloseDate        *string          `json:"close_date"`
	Explanation      *string          `json:"explanation"`
	// ExplanationEntities are [] without an explanation.
	ExplanationEntities []textEntityJSON `json:"explanation_entities"`
	Options             []pollOptionJSON `json:"options"`
	OptionsTotal        int              `json:"options_total"`
}

type taskJSON struct {
	ID       int64            `json:"id"`
	Text     string           `json:"text"`
	Entities []textEntityJSON `json:"entities"`
	Done     bool             `json:"done"`
	DoneBy   *chatRefJSON     `json:"done_by"`
	DoneAt   *string          `json:"done_at"`
}

type checklistJSON struct {
	Kind          string     `json:"kind"`
	Tasks         []taskJSON `json:"tasks"`
	TasksTotal    int        `json:"tasks_total"`
	OthersCanAdd  bool       `json:"others_can_add"`
	OthersCanMark bool       `json:"others_can_mark"`
}

type storyJSON struct {
	Kind    string       `json:"kind"`
	Poster  *chatRefJSON `json:"poster"`
	StoryID int64        `json:"story_id"`
	// Username is the poster's active username, for a link to the
	// story; nil without one or when the viewer may not see the
	// poster.
	Username *string `json:"username"`
}

type invoiceJSON struct {
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Currency    string         `json:"currency"`
	TotalAmount int64          `json:"total_amount"`
	IsTest      bool           `json:"is_test"`
	Photo       *chatMediaJSON `json:"photo"`
}

type prizeJSON struct {
	PremiumMonths *int64 `json:"premium_months,omitempty"`
	Stars         *int64 `json:"stars,omitempty"`
}

type giveawayJSON struct {
	Kind                 string        `json:"kind"`
	Prize                *prizeJSON    `json:"prize"`
	WinnerCount          int64         `json:"winner_count"`
	WinnersSelectionDate *string       `json:"winners_selection_date"`
	OnlyNewMembers       bool          `json:"only_new_members"`
	PrizeDescription     string        `json:"prize_description"`
	Chats                []chatRefJSON `json:"chats"`
	ChatsTotal           int           `json:"chats_total"`
}

type callJSON struct {
	Kind              string        `json:"kind"`
	Video             bool          `json:"video"`
	Group             bool          `json:"group"`
	Duration          int64         `json:"duration"`
	Outcome           string        `json:"outcome"`
	Participants      []chatRefJSON `json:"participants"`
	ParticipantsTotal int           `json:"participants_total"`
}

type paidItemJSON struct {
	Kind  string         `json:"kind"`
	Media *chatMediaJSON `json:"media"`
}

type paidMediaJSON struct {
	Kind      string         `json:"kind"`
	StarCount int64          `json:"star_count"`
	Items     []paidItemJSON `json:"items"`
}

type placeholderJSON struct {
	Kind string `json:"kind"`
}

// formattedText reads a content_extra text that is either a string or
// a formatted text {text, entities}.
func formattedText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var ft struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(raw, &ft)
	return ft.Text
}

// messageFile is the message's other file of this role and position.
func messageFile(m logdb.Message, role string, pos int) (logdb.FileRef, bool) {
	for _, f := range m.Files {
		if f.Role == role && f.Position == pos {
			return f.File, true
		}
	}
	return logdb.FileRef{}, false
}

// callOutcome is how a call ended, as CallDetail names it.
func callOutcome(x *kindExtra) string {
	if x.Group {
		switch {
		case x.Active:
			return "active"
		case x.Missed:
			return "missed"
		}
		return "ended"
	}
	switch x.DiscardReason {
	case "missed", "declined", "disconnected", "hung_up":
		return x.DiscardReason
	case "upgrade_to_group_call":
		return "upgraded"
	}
	return "ended"
}

// detail is a message's MessageDetail, or nil (see MessageKinds.detail).
func (v view) detail(k kinds, m logdb.Message) any {
	return v.kindDetail(k, m, m.ContentType, k.extra[m.RowID], len(m.Edits))
}

// editDetail is the MessageDetail of a message's i-th earlier version,
// or nil (see Edit.detail). The other files (a game's or an invoice's
// photo, paid media's items) are the message's as it is now, so a
// version shows none; a live location's updates are the versions
// before it.
func (v view) editDetail(k kinds, m logdb.Message, i int) any {
	var x *kindExtra
	if xs := k.edits[m.RowID]; xs != nil {
		x = xs[i]
	}
	return v.kindDetail(k, logdb.Message{}, m.Edits[i].ContentType, x, i)
}

// kindDetail is the MessageDetail of a ctype content with x as its
// parsed content_extra, the other files of m and updates earlier
// positions of a live location; nil without one.
func (v view) kindDetail(k kinds, m logdb.Message, ctype string, x *kindExtra, updates int) any {
	switch ctype {
	case "rich", "widget", "expired", "unsupported":
		return placeholderJSON{Kind: ctype}
	}
	if x == nil {
		return nil
	}
	switch ctype {
	case "location":
		if x.Lat == nil || x.Lon == nil {
			return nil
		}
		j := locationJSON{Kind: ctype, Lat: *x.Lat, Lon: *x.Lon}
		if x.Accuracy != 0 {
			a := x.Accuracy
			j.Accuracy = &a
		}
		if x.Live {
			j.Live = &liveJSON{Period: x.LivePeriod, Heading: x.Heading,
				ProximityRadius: x.ProximityRadius, Updates: updates}
		}
		return j
	case "venue":
		if x.Lat == nil || x.Lon == nil {
			return nil
		}
		return venueJSON{Kind: ctype, Lat: *x.Lat, Lon: *x.Lon, Title: x.Title, Address: x.Address,
			Provider: x.Provider, VenueID: x.VenueID, VenueType: x.VenueType}
	case "contact":
		j := contactJSON{Kind: ctype, FirstName: x.FirstName, LastName: x.LastName}
		// A phone number follows the rule for users' phones: viewers
		// that see users in full only. The vCard holds it too.
		if v.full() {
			if x.Phone != "" {
				p := x.Phone
				j.Phone = &p
			}
			if x.Vcard != "" {
				c := x.Vcard
				j.Vcard = &c
			}
		}
		if x.UserID > 0 {
			j.User = v.refPtr(k, x.UserID)
		}
		return j
	case "dice":
		return diceJSON{Kind: ctype, Emoji: x.Emoji, Value: x.Value, Stake: x.Stake}
	case "game":
		j := gameJSON{Kind: ctype, Title: x.Title, Description: formattedText(x.Description),
			ShortName: x.ShortName}
		if f, ok := messageFile(m, "game", 0); ok {
			j.Photo = v.media("photo", f, nil)
		} else if f, ok := messageFile(m, "game", 1); ok {
			j.Photo = v.media("animation", f, nil)
		}
		return j
	case "poll":
		p := k.polls[x.pollID()]
		if p == nil {
			return nil
		}
		j := pollJSON{Kind: ctype, Question: p.Question, Type: p.Type, Anonymous: p.Anonymous,
			Multiple: p.Multiple, Closed: pollClosed(p.Closed, p.CloseDate, v.s.clock().Unix()),
			TotalVoters: p.TotalVoters, CloseDate: isoPtr(p.CloseDate), Options: []pollOptionJSON{},
			OptionsTotal:        len(p.Options),
			QuestionEntities:    v.entities(k, p.Question, p.QuestionEntities, "poll question"),
			ExplanationEntities: v.entities(k, p.Explanation, p.ExplanationEntities, "poll explanation")}
		if p.Explanation != "" {
			e := p.Explanation
			j.Explanation = &e
		}
		correct := map[int]bool{}
		for _, c := range p.Correct {
			correct[c] = true
		}
		for _, o := range p.Options {
			if len(j.Options) == listCap {
				break
			}
			j.Options = append(j.Options, pollOptionJSON{Text: o.Text, Voters: o.Voters,
				Percent: o.Percent, Correct: p.Type == "quiz" && correct[o.Position]})
		}
		return j
	case "checklist":
		j := checklistJSON{Kind: ctype, Tasks: []taskJSON{}, TasksTotal: len(x.Tasks),
			OthersCanAdd: x.OthersCanAdd, OthersCanMark: x.OthersCanMark}
		for _, t := range x.Tasks {
			if len(j.Tasks) == listCap {
				break
			}
			tj := taskJSON{ID: t.ID, Text: t.Text,
				Entities: v.entities(k, t.Text, t.Entities, "checklist task"),
				Done:     t.CompletedBy != nil || t.CompletionDate != 0, DoneAt: isoPtr(t.CompletionDate)}
			if t.CompletedBy != nil {
				tj.DoneBy = v.refPtr(k, t.CompletedBy.id())
			}
			j.Tasks = append(j.Tasks, tj)
		}
		return j
	case "story":
		return storyJSON{Kind: ctype, Poster: v.refPtr(k, x.PosterChatID), StoryID: x.StoryID,
			Username: refUsername(k, x.PosterChatID)}
	case "invoice":
		j := invoiceJSON{Kind: ctype, Title: x.Title, Description: formattedText(x.Description),
			Currency: x.Currency, TotalAmount: x.TotalAmount, IsTest: x.IsTest}
		if f, ok := messageFile(m, "invoice", 0); ok {
			j.Photo = v.media("photo", f, nil)
		}
		return j
	case "giveaway":
		chats := x.giveawayChats()
		j := giveawayJSON{Kind: ctype, WinnerCount: x.WinnerCount,
			WinnersSelectionDate: isoPtr(x.WinnersSelectionDate), OnlyNewMembers: x.OnlyNewMembers,
			PrizeDescription: x.PrizeDescription, Chats: v.refList(k, chats), ChatsTotal: len(chats)}
		if p := x.Prize; p != nil && (p.PremiumMonths != nil || p.Stars != nil) {
			j.Prize = &prizeJSON{PremiumMonths: p.PremiumMonths, Stars: p.Stars}
		}
		return j
	case "call":
		ids := make([]int64, 0, len(x.Participants))
		for _, p := range x.Participants {
			ids = append(ids, p.id())
		}
		return callJSON{Kind: ctype, Video: x.Video, Group: x.Group, Duration: x.Duration,
			Outcome: callOutcome(x), Participants: v.refList(k, ids), ParticipantsTotal: len(ids)}
	case "paid_media":
		j := paidMediaJSON{Kind: ctype, StarCount: x.StarCount, Items: []paidItemJSON{}}
		for i, it := range x.Items {
			item := paidItemJSON{Kind: it.Kind}
			if f, ok := messageFile(m, "paid", i); ok && (it.Kind == "photo" || it.Kind == "video") {
				item.Media = v.media(it.Kind, f, nil)
			}
			j.Items = append(j.Items, item)
		}
		return j
	}
	return nil
}
