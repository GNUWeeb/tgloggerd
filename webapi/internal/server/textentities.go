// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"unicode/utf16"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// The text entities of messages, edits, polls and checklist tasks
// (webapi/openapi.yaml: TextEntity; web/docs/message-kinds.md, "Text
// formatting"): Telegram's formatting of a text as the daemon stores
// it (format_entities_json in src/tgloggerd/MessageExtract.cpp), a
// JSON array of {offset, length, type, ...} with offsets and lengths
// in UTF-16 code units. They are sent as stored but for the people
// they name: a mention_name's user_id only when the viewer may see
// that user, and an @username mention resolved to the user holding
// that username only for a viewer that may see that user. Both are
// resolved in one batch per page of messages (mentionCollector.load).

// storedEntity is one entity as stored.
type storedEntity struct {
	Offset         *int64          `json:"offset"`
	Length         *int64          `json:"length"`
	Type           string          `json:"type"`
	URL            *string         `json:"url"`
	UserID         *int64          `json:"user_id"`
	Language       *string         `json:"language"`
	CustomEmojiID  json.RawMessage `json:"custom_emoji_id"`
	MediaTimestamp *int64          `json:"media_timestamp"`
}

// textEntityJSON is a TextEntity. UserID is a mention's: absent for
// other types and for an @username the viewer gets no user for, null
// for a mention_name of a user it may not see.
type textEntityJSON struct {
	Offset         int64           `json:"offset"`
	Length         int64           `json:"length"`
	Type           string          `json:"type"`
	URL            *string         `json:"url,omitempty"`
	UserID         json.RawMessage `json:"user_id,omitempty"`
	Language       *string         `json:"language,omitempty"`
	CustomEmojiID  string          `json:"custom_emoji_id,omitempty"`
	MediaTimestamp *int64          `json:"media_timestamp,omitempty"`
}

// malformedLogged is set once a malformed entities value was logged:
// one row's bad value would otherwise be logged on every page that
// shows it.
var malformedLogged atomic.Bool

// entityEmojiID is a custom emoji's id as a string: the daemon stores
// it as a JSON number, which a client's double would round, but a
// string is taken too. ok is false for anything else.
func entityEmojiID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", true
	}
	var s string
	if raw[0] == '"' {
		if json.Unmarshal(raw, &s) != nil {
			return "", false
		}
		return s, true
	}
	var n json.Number
	if json.Unmarshal(raw, &n) != nil {
		return "", false
	}
	s = n.String()
	if strings.ContainsAny(s, ".eE+-") {
		return "", false
	}
	return s, true
}

// parseEntities reads a stored entities value; none for NULL (plain
// text). A value that is not an array of entities with an offset, a
// length and a type gives none, as a whole: half a formatting would
// mislead more than none.
func (s *Server) parseEntities(raw []byte, what string) []storedEntity {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var es []storedEntity
	bad := json.Unmarshal(raw, &es) != nil
	for _, e := range es {
		if bad {
			break
		}
		_, idOK := entityEmojiID(e.CustomEmojiID)
		bad = e.Offset == nil || e.Length == nil || *e.Offset < 0 || *e.Length < 0 ||
			e.Type == "" || !idOK
	}
	if bad {
		if malformedLogged.CompareAndSwap(false, true) {
			s.log.Debug("malformed text entities, sent as none", "of", what,
				"value", string(raw))
		}
		return nil
	}
	return es
}

// mentionUsername is the username an @username mention names: the
// entity's span of text without its @, "" when the span is not in the
// text or not a username.
func mentionUsername(text string, e storedEntity) string {
	u := utf16.Encode([]rune(text))
	off, n := *e.Offset, *e.Length
	if off > int64(len(u)) || n > int64(len(u))-off {
		return ""
	}
	name, ok := strings.CutPrefix(string(utf16.Decode(u[off:off+n])), "@")
	if !ok || len(name) == 0 || len(name) > 32 {
		return ""
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return ""
		}
	}
	return name
}

// textMentions are the people a page's entities name, as the viewer may
// see them.
type textMentions struct {
	// visible are the users named (by mention_name or as an
	// username's owner) that the viewer sees.
	visible map[int64]bool
	// owners are the usernames mentioned (lowercased) that a user
	// holds, and who.
	owners map[string]int64
}

// mentionCollector gathers what a page's entities name before the
// lookups.
type mentionCollector struct {
	s     *Server
	ids   []int64
	names []string
}

func (c *mentionCollector) add(text string, raw []byte, what string) {
	for _, e := range c.s.parseEntities(raw, what) {
		switch e.Type {
		case "mention_name":
			if e.UserID != nil && *e.UserID > 0 {
				c.ids = append(c.ids, *e.UserID)
			}
		case "mention":
			if n := mentionUsername(text, e); n != "" {
				c.names = append(c.names, n)
			}
		}
	}
}

// load resolves what c gathered: the usernames' owners in one
// query, then which of the users named the viewer sees in one more;
// none when nothing is named.
func (c *mentionCollector) load(ctx context.Context, sc scope.Scope) (textMentions, error) {
	m := textMentions{visible: map[int64]bool{}, owners: map[string]int64{}}
	var err error
	if m.owners, err = logdb.UsernameOwners(ctx, c.s.ro, c.names); err != nil {
		return m, err
	}
	ids := c.ids
	for _, id := range m.owners {
		ids = append(ids, id)
	}
	m.visible, err = logdb.VisibleUserIDs(ctx, c.s.ro, sc, ids)
	return m, err
}

// entities is a text's entities for the viewer: [] for plain text.
func (v view) entities(k kinds, text string, raw []byte, what string) []textEntityJSON {
	out := []textEntityJSON{}
	for _, e := range v.s.parseEntities(raw, what) {
		j := textEntityJSON{Offset: *e.Offset, Length: *e.Length, Type: e.Type, URL: e.URL,
			Language: e.Language, MediaTimestamp: e.MediaTimestamp}
		j.CustomEmojiID, _ = entityEmojiID(e.CustomEmojiID)
		switch e.Type {
		case "mention_name":
			j.UserID = json.RawMessage("null")
			if e.UserID != nil && k.mentions.visible[*e.UserID] {
				j.UserID, _ = json.Marshal(*e.UserID)
			}
		case "mention":
			name := strings.ToLower(mentionUsername(text, e))
			if id, ok := k.mentions.owners[name]; ok && name != "" && k.mentions.visible[id] {
				j.UserID, _ = json.Marshal(id)
			}
		}
		out = append(out, j)
	}
	return out
}

// messageEntities are a message's (or one of its versions') entities
// for the viewer. A service message gets none: its text is the
// logger's description, rebuilt for the viewer (service.go).
func (v view) messageEntities(k kinds, m logdb.Message, text string, raw []byte) []textEntityJSON {
	if m.ContentType == "service" {
		return []textEntityJSON{}
	}
	return v.entities(k, text, raw, "message")
}
