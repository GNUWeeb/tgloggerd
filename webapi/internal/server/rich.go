// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf16"

	"tgloggerd/webapi/internal/logdb"
)

// A rich message's detail (webapi/openapi.yaml: RichDetail;
// web/docs/message-kinds.md, "Rich messages"): its page blocks,
// converted from the TDLib JSON the daemon keeps under content_extra's
// "tdlib" key (docs/design/message-types.md section 4.2, decision
// 2026-11-19-msg-rich-raw-json) into a shape of their own. A block's
// rich text becomes a text with TextEntity formatting, so the client's
// text entities renderer draws it; the people it names are resolved
// through the request scope with the page's other entities. A block's
// media is the message's file of role rich at the block's position:
// the daemon numbers them in order of appearance, depth first, counting
// only the media that have a file (message_extra_files in
// src/tgloggerd/MessageExtract.cpp), and richConv counts them the same
// way. A video's or an animation's still thumbnail is the file of role
// thumbnail at the same position (section 4.3). A block type this service does not know is sent as type unknown
// with its class name.

// richBlockTypes are the block types RichBlock names: TDLib's classes
// in snake_case without their pageBlock prefix.
var richBlockTypes = map[string]bool{}

func init() {
	for _, t := range []string{"title", "subtitle", "author_date", "header", "subheader",
		"section_heading", "kicker", "paragraph", "preformatted", "footer", "thinking", "divider",
		"mathematical_expression", "anchor", "list", "block_quote", "expandable_block_quote",
		"pull_quote", "photo", "video", "animation", "audio", "document", "voice_note", "cover",
		"embedded", "embedded_post", "collage", "slideshow", "chat_link", "table", "details",
		"related_articles", "map", "button_row"} {
		richBlockTypes[t] = true
	}
}

// richTextJSON is a RichText. stored are its entities before the
// viewer's scope applies (view.richEntities fills Entities from them).
type richTextJSON struct {
	Text     string           `json:"text"`
	Entities []textEntityJSON `json:"entities"`
	stored   []richEntity
}

// richEntity is an entity of a rich text and, for a mention_name, the
// user it names (resolved for the viewer as message entities are).
type richEntity struct {
	textEntityJSON
	userID int64
}

type richCaptionJSON struct {
	Text   *richTextJSON `json:"text"`
	Credit *richTextJSON `json:"credit"`
}

type richListItemJSON struct {
	Label    string           `json:"label"`
	Blocks   []*richBlockJSON `json:"blocks"`
	Checkbox bool             `json:"checkbox"`
	Checked  bool             `json:"checked"`
}

type richCellJSON struct {
	Text    *richTextJSON `json:"text"`
	Header  bool          `json:"header"`
	Colspan int64         `json:"colspan"`
	Rowspan int64         `json:"rowspan"`
	Align   string        `json:"align"`
	Valign  string        `json:"valign"`
}

type richArticleJSON struct {
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Author      string         `json:"author"`
	Date        *string        `json:"date"`
	Photo       *chatMediaJSON `json:"photo,omitempty"`
}

type richButtonJSON struct {
	Text *richTextJSON `json:"text"`
	URL  string        `json:"url,omitempty"`
}

// richBlockJSON is a RichBlock; the fields a type does not use stay
// zero and are left out.
type richBlockJSON struct {
	Type       string              `json:"type"`
	Text       *richTextJSON       `json:"text,omitempty"`
	Credit     *richTextJSON       `json:"credit,omitempty"`
	Caption    *richCaptionJSON    `json:"caption,omitempty"`
	Size       *int64              `json:"size,omitempty"`
	Language   *string             `json:"language,omitempty"`
	Date       **string            `json:"date,omitempty"`
	Expression *string             `json:"expression,omitempty"`
	Name       *string             `json:"name,omitempty"`
	Blocks     []*richBlockJSON    `json:"blocks,omitempty"`
	Items      []*richListItemJSON `json:"items,omitempty"`
	Media      *chatMediaJSON      `json:"media,omitempty"`
	Spoiler    *bool               `json:"spoiler,omitempty"`
	Autoplay   *bool               `json:"autoplay,omitempty"`
	Loop       *bool               `json:"loop,omitempty"`
	URL        *string             `json:"url,omitempty"`
	Author     *string             `json:"author,omitempty"`
	Title      *string             `json:"title,omitempty"`
	Username   *string             `json:"username,omitempty"`
	Width      *int64              `json:"width,omitempty"`
	Height     *int64              `json:"height,omitempty"`
	Rows       [][]*richCellJSON   `json:"rows,omitempty"`
	Bordered   *bool               `json:"bordered,omitempty"`
	Striped    *bool               `json:"striped,omitempty"`
	Open       *bool               `json:"open,omitempty"`
	Articles   []*richArticleJSON  `json:"articles,omitempty"`
	Lat        *float64            `json:"lat,omitempty"`
	Lon        *float64            `json:"lon,omitempty"`
	Zoom       *int64              `json:"zoom,omitempty"`
	Buttons    []*richButtonJSON   `json:"buttons,omitempty"`
	Align      *string             `json:"align,omitempty"`
}

// richDetailJSON is a RichDetail. Blocks is nil (null) when the layout
// was not kept.
type richDetailJSON struct {
	Kind   string           `json:"kind"`
	Blocks []*richBlockJSON `json:"blocks"`
	IsRTL  bool             `json:"is_rtl"`
	IsFull bool             `json:"is_full"`
}

// richMedia is a block's medium, set for the viewer at render time
// from the message's file of role rich at pos.
type richMedia struct {
	pos   int
	ctype string // as the media detail names it: photo, video, ...
	extra []byte // its detail, as content_extra's media keys
	set   func(*chatMediaJSON)
}

// richDoc is a rich message (or one of its versions) as parsed for a
// page: the blocks, and the texts and media to complete for the
// viewer.
type richDoc struct {
	detail richDetailJSON
	texts  []*richTextJSON
	media  []richMedia
	// users and usernames are the people the texts name.
	users     []int64
	usernames []string
}

// tdObj is a TDLib JSON object.
type tdObj = map[string]any

func tdObject(v any) tdObj {
	o, _ := v.(map[string]any)
	return o
}

func (c *richConv) str(o tdObj, key string) string {
	s, _ := o[key].(string)
	return s
}

func tdInt(o tdObj, key string) int64 {
	switch n := o[key].(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
		if f, err := n.Float64(); err == nil {
			return int64(f)
		}
	case string: // TDLib's int64 fields are strings
		var i json.Number = json.Number(n)
		if v, err := i.Int64(); err == nil {
			return v
		}
	}
	return 0
}

func tdFloat(o tdObj, key string) float64 {
	if n, ok := o[key].(json.Number); ok {
		f, _ := n.Float64()
		return f
	}
	return 0
}

func tdBool(o tdObj, key string) bool {
	b, _ := o[key].(bool)
	return b
}

func tdList(o tdObj, key string) []any {
	l, _ := o[key].([]any)
	return l
}

func tdType(o tdObj) string {
	s, _ := o["@type"].(string)
	return s
}

// tdFile is o[key] when it is a file object, else nil.
func tdFile(o tdObj, key string) tdObj {
	f := tdObject(o[key])
	if tdType(f) != "file" {
		return nil
	}
	return f
}

// snakeClass is a TDLib class in snake_case without prefix:
// pageBlockSectionHeading -> section_heading.
func snakeClass(cls, prefix string) string {
	s, _ := strings.CutPrefix(cls, prefix)
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// richSafeURL is u when it is a link a page may open (http, https, a
// Telegram link, mail or a phone number), else "": an archive shows
// what someone else wrote, and a javascript: link must not reach the
// page.
func richSafeURL(u string) string {
	l := strings.ToLower(strings.TrimSpace(u))
	for _, p := range []string{"http://", "https://", "tg://", "mailto:", "tel:"} {
		if strings.HasPrefix(l, p) {
			return strings.TrimSpace(u)
		}
	}
	return ""
}

// richConv converts one message's page blocks.
type richConv struct {
	doc *richDoc
	pos int // the next medium's position, as the daemon counts them
}

// parseRich reads a rich message's content_extra; a value with no
// page (over the cap, or not read again since before the mapping) has
// nil blocks.
func parseRich(raw []byte) *richDoc {
	doc := &richDoc{detail: richDetailJSON{Kind: "rich", IsFull: true}}
	if len(raw) == 0 {
		return doc
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	var top any
	if d.Decode(&top) != nil {
		return doc
	}
	x := tdObject(top)
	doc.detail.IsRTL = tdBool(x, "is_rtl")
	msg := tdObject(tdObject(x["tdlib"])["message"])
	if msg == nil {
		return doc
	}
	doc.detail.IsRTL = tdBool(msg, "is_rtl")
	if f, ok := msg["is_full"].(bool); ok {
		doc.detail.IsFull = f
	}
	c := &richConv{doc: doc}
	doc.detail.Blocks = c.blocks(tdList(msg, "blocks"))
	return doc
}

func (c *richConv) blocks(l []any) []*richBlockJSON {
	out := []*richBlockJSON{}
	for _, v := range l {
		if o := tdObject(v); o != nil {
			out = append(out, c.block(o))
		}
	}
	return out
}

// medium registers a block's file, when there is one, at the next
// position, and has set fill its field for the viewer.
func (c *richConv) medium(f tdObj, ctype string, extra contentExtra, set func(*chatMediaJSON)) {
	if f == nil {
		return
	}
	raw, _ := json.Marshal(extra)
	c.doc.media = append(c.doc.media, richMedia{pos: c.pos, ctype: ctype, extra: raw, set: set})
	c.pos++
}

// minithumb is an object's minithumbnail as content_extra keeps it.
func minithumb(o tdObj) *struct {
	Data string `json:"data"`
} {
	m := tdObject(o["minithumbnail"])
	if m == nil {
		return nil
	}
	s, _ := m["data"].(string)
	return &struct {
		Data string `json:"data"`
	}{Data: s}
}

// photo registers a photo's largest size, as photo_best_size picks it.
func (c *richConv) photo(p tdObj, set func(*chatMediaJSON)) {
	var best tdObj
	bestPx := int64(-1)
	var x contentExtra
	for _, v := range tdList(p, "sizes") {
		s := tdObject(v)
		f := tdFile(s, "photo")
		if f == nil {
			continue
		}
		if px := tdInt(s, "width") * tdInt(s, "height"); px > bestPx {
			bestPx, best = px, f
			x.Width, x.Height = int(tdInt(s, "width")), int(tdInt(s, "height"))
		}
	}
	x.Minithumbnail = minithumb(p)
	c.medium(best, "photo", x, set)
}

func ptr[T any](v T) *T { return &v }

func (c *richConv) text(v any) *richTextJSON {
	t := &richTextJSON{Entities: []textEntityJSON{}}
	var b strings.Builder
	n := int64(0) // UTF-16 length so far
	write := func(s string) {
		b.WriteString(s)
		n += int64(len(utf16.Encode([]rune(s))))
	}
	var walk func(o tdObj)
	// wrap writes o's text inside entity e; its slot is taken before
	// the entities inside, so the outer one comes first.
	wrap := func(o tdObj, e richEntity) {
		start, slot := n, len(t.stored)
		t.stored = append(t.stored, richEntity{})
		walk(tdObject(o["text"]))
		if n > start && e.Type != "" {
			e.Offset, e.Length = start, n-start
			t.stored[slot] = e
		}
	}
	walk = func(o tdObj) {
		if o == nil {
			return
		}
		ent := func(typ string) richEntity { return richEntity{textEntityJSON: textEntityJSON{Type: typ}} }
		link := func(u string) richEntity {
			if u = richSafeURL(u); u == "" {
				return richEntity{}
			}
			e := ent("text_url")
			e.URL = &u
			return e
		}
		switch tdType(o) {
		case "richTextPlain":
			write(c.str(o, "text"))
		case "richTexts":
			for _, x := range tdList(o, "texts") {
				walk(tdObject(x))
			}
		case "richTextBold":
			wrap(o, ent("bold"))
		case "richTextItalic":
			wrap(o, ent("italic"))
		case "richTextUnderline":
			wrap(o, ent("underline"))
		case "richTextStrikethrough":
			wrap(o, ent("strikethrough"))
		case "richTextSpoiler":
			wrap(o, ent("spoiler"))
		case "richTextSubscript":
			wrap(o, ent("subscript"))
		case "richTextSuperscript":
			wrap(o, ent("superscript"))
		case "richTextMarked":
			wrap(o, ent("marked"))
		case "richTextFixed":
			wrap(o, ent("code"))
		case "richTextHashtag":
			wrap(o, ent("hashtag"))
		case "richTextCashtag":
			wrap(o, ent("cashtag"))
		case "richTextBankCardNumber":
			wrap(o, ent("bank_card_number"))
		case "richTextBotCommand":
			wrap(o, ent("bot_command"))
		case "richTextEmailAddress":
			wrap(o, ent("email"))
		case "richTextPhoneNumber":
			wrap(o, ent("phone_number"))
		case "richTextUrl":
			wrap(o, link(c.str(o, "url")))
		case "richTextReferenceLink", "richTextAnchorLink":
			wrap(o, link(c.str(o, "url")))
		case "richTextMentionName":
			e := ent("mention_name")
			e.userID = tdInt(o, "user_id")
			if e.userID > 0 {
				c.doc.users = append(c.doc.users, e.userID)
			}
			wrap(o, e)
		case "richTextMention":
			// An @username shown as such is a mention, which the
			// page resolves to its user; other text links to it.
			u := c.str(o, "username")
			start, at := n, b.Len()
			walk(tdObject(o["text"]))
			span := b.String()[at:]
			if n == start || u == "" {
				break
			}
			e := ent("mention")
			if span != "@"+u {
				e = link("https://t.me/" + u)
			} else {
				c.doc.usernames = append(c.doc.usernames, u)
			}
			e.Offset, e.Length = start, n-start
			t.stored = append(t.stored, e)
		case "richTextCustomEmoji":
			start := n
			write(c.str(o, "alternative_text"))
			if id := tdInt(o, "custom_emoji_id"); id != 0 && n > start {
				e := ent("custom_emoji")
				e.CustomEmojiID = itoa(id)
				e.Offset, e.Length = start, n-start
				t.stored = append(t.stored, e)
			}
		case "richTextMathematicalExpression":
			start := n
			write(c.str(o, "expression"))
			if n > start {
				e := ent("code")
				e.Offset, e.Length = start, n-start
				t.stored = append(t.stored, e)
			}
		case "richTextButton":
			btn := tdObject(o["button"])
			wrap(btn, link(richButtonURL(btn)))
		case "richTextDateTime", "richTextTonAddress", "richTextDiff", "richTextReference":
			// The text as shown; a diff shows its new text.
			walk(tdObject(o["text"]))
		default:
			// An icon or an anchor adds no text; an unknown run
			// shows its text, if it has one.
			walk(tdObject(o["text"]))
		}
	}
	walk(tdObject(v))
	t.Text = b.String()
	t.stored = slices.DeleteFunc(t.stored, func(e richEntity) bool { return e.Type == "" })
	// By offset, outer entities before the ones they hold, as
	// Telegram sorts them.
	slices.SortStableFunc(t.stored, func(a, b richEntity) int {
		if a.Offset != b.Offset {
			return int(a.Offset - b.Offset)
		}
		return int(b.Length - a.Length)
	})
	c.doc.texts = append(c.doc.texts, t)
	return t
}

func itoa(i int64) string {
	b, _ := json.Marshal(i)
	return string(b)
}

// richButtonURL is where a button goes, for a link button; "" for the
// others.
func richButtonURL(btn tdObj) string {
	t := tdObject(btn["type"])
	switch tdType(t) {
	case "inlineKeyboardButtonTypeUrl", "inlineKeyboardButtonTypeLoginUrl",
		"inlineKeyboardButtonTypeWebApp":
		s, _ := t["url"].(string)
		return richSafeURL(s)
	}
	return ""
}

func (c *richConv) caption(v any) *richCaptionJSON {
	o := tdObject(v)
	return &richCaptionJSON{Text: c.text(o["text"]), Credit: c.text(o["credit"])}
}

// align is a PageBlockHorizontalAlignment or PageBlockVerticalAlignment
// as RichBlock names it.
func align(v any, prefix, def string) string {
	if s := snakeClass(tdType(tdObject(v)), prefix); s != "" {
		return s
	}
	return def
}

func (c *richConv) block(o tdObj) *richBlockJSON {
	cls := tdType(o)
	b := &richBlockJSON{Type: snakeClass(cls, "pageBlock")}
	if !strings.HasPrefix(cls, "pageBlock") || !richBlockTypes[b.Type] {
		// pageBlockUnsupported, and the types added after this
		// service: a block it does not draw. The daemon lists no
		// media for a type it does not know, so none are counted.
		return &richBlockJSON{Type: "unknown", Name: ptr(b.Type)}
	}
	setMedia := func(m *chatMediaJSON) { b.Media = m }
	switch b.Type {
	case "title", "subtitle", "header", "subheader", "kicker", "paragraph", "footer", "thinking":
		key := map[string]string{"title": "title", "subtitle": "subtitle", "header": "header",
			"subheader": "subheader", "kicker": "kicker", "footer": "footer"}[b.Type]
		if key == "" {
			key = "text"
		}
		b.Text = c.text(o[key])
	case "section_heading":
		b.Text = c.text(o["text"])
		b.Size = ptr(tdInt(o, "size"))
	case "author_date":
		b.Text = c.text(o["author"])
		b.Date = ptr(isoPtr(tdInt(o, "publish_date")))
	case "preformatted":
		b.Text = c.text(o["text"])
		b.Language = ptr(c.str(o, "language"))
	case "divider":
	case "mathematical_expression":
		b.Expression = ptr(c.str(o, "expression"))
	case "anchor":
		b.Name = ptr(c.str(o, "name"))
	case "list":
		b.Items = []*richListItemJSON{}
		for _, v := range tdList(o, "items") {
			it := tdObject(v)
			if it == nil {
				continue
			}
			b.Items = append(b.Items, &richListItemJSON{Label: c.str(it, "label"),
				Blocks: c.blocks(tdList(it, "blocks")), Checkbox: tdBool(it, "has_checkbox"),
				Checked: tdBool(it, "is_checked")})
		}
	case "block_quote":
		b.Blocks = c.blocks(tdList(o, "blocks"))
		b.Credit = c.text(o["credit"])
		if b.Blocks == nil {
			b.Blocks = []*richBlockJSON{}
		}
	case "expandable_block_quote", "pull_quote":
		b.Text = c.text(o["text"])
		b.Credit = c.text(o["credit"])
	case "photo":
		c.photo(tdObject(o["photo"]), setMedia)
		b.Caption = c.caption(o["caption"])
		b.Spoiler = ptr(tdBool(o, "has_spoiler"))
		if u := richSafeURL(c.str(o, "url")); u != "" {
			b.URL = &u
		}
	case "video", "animation":
		m := tdObject(o[b.Type])
		x := contentExtra{Width: int(tdInt(m, "width")), Height: int(tdInt(m, "height")),
			Duration: int(tdInt(m, "duration")), Mime: c.str(m, "mime_type"),
			Minithumbnail: minithumb(m)}
		c.medium(tdFile(m, b.Type), b.Type, x, setMedia)
		b.Caption = c.caption(o["caption"])
		b.Spoiler = ptr(tdBool(o, "has_spoiler"))
		b.Autoplay = ptr(tdBool(o, "need_autoplay"))
		if b.Type == "video" {
			b.Loop = ptr(tdBool(o, "is_looped"))
		}
	case "audio":
		m := tdObject(o["audio"])
		x := contentExtra{Duration: int(tdInt(m, "duration")), Mime: c.str(m, "mime_type"),
			Title: c.str(m, "title"), Performer: c.str(m, "performer")}
		c.medium(tdFile(m, "audio"), "audio", x, setMedia)
		b.Caption = c.caption(o["caption"])
	case "document":
		m := tdObject(o["document"])
		x := contentExtra{Mime: c.str(m, "mime_type"), Minithumbnail: minithumb(m)}
		c.medium(tdFile(m, "document"), "document", x, setMedia)
		b.Caption = c.caption(o["caption"])
	case "voice_note":
		m := tdObject(o["voice_note"])
		x := contentExtra{Duration: int(tdInt(m, "duration")), Mime: c.str(m, "mime_type"),
			Waveform: c.str(m, "waveform")}
		c.medium(tdFile(m, "voice"), "voice", x, setMedia)
		b.Caption = c.caption(o["caption"])
	case "cover":
		b.Blocks = []*richBlockJSON{}
		if cv := tdObject(o["cover"]); cv != nil {
			b.Blocks = append(b.Blocks, c.block(cv))
		}
	case "embedded":
		if u := richSafeURL(c.str(o, "url")); u != "" {
			b.URL = &u
		}
		c.photo(tdObject(o["poster_photo"]), setMedia)
		b.Width, b.Height = ptr(tdInt(o, "width")), ptr(tdInt(o, "height"))
		b.Caption = c.caption(o["caption"])
	case "embedded_post":
		if u := richSafeURL(c.str(o, "url")); u != "" {
			b.URL = &u
		}
		b.Author = ptr(c.str(o, "author"))
		c.photo(tdObject(o["author_photo"]), setMedia)
		b.Date = ptr(isoPtr(tdInt(o, "date")))
		b.Blocks = c.blocks(tdList(o, "blocks"))
		b.Caption = c.caption(o["caption"])
	case "collage", "slideshow":
		b.Blocks = c.blocks(tdList(o, "blocks"))
		b.Caption = c.caption(o["caption"])
	case "chat_link":
		b.Title = ptr(c.str(o, "title"))
		b.Username = ptr(c.str(o, "username"))
	case "table":
		b.Text = c.text(o["caption"])
		b.Rows = [][]*richCellJSON{}
		for _, r := range tdList(o, "cells") {
			row := []*richCellJSON{}
			l, _ := r.([]any)
			for _, v := range l {
				cell := tdObject(v)
				if cell == nil {
					continue
				}
				rc := &richCellJSON{Text: c.text(cell["text"]), Header: tdBool(cell, "is_header"),
					Colspan: max(tdInt(cell, "colspan"), 1), Rowspan: max(tdInt(cell, "rowspan"), 1),
					Align:  align(cell["align"], "pageBlockHorizontalAlignment", "left"),
					Valign: align(cell["valign"], "pageBlockVerticalAlignment", "top")}
				row = append(row, rc)
			}
			b.Rows = append(b.Rows, row)
		}
		b.Bordered, b.Striped = ptr(tdBool(o, "is_bordered")), ptr(tdBool(o, "is_striped"))
	case "details":
		b.Text = c.text(o["header"])
		b.Blocks = c.blocks(tdList(o, "blocks"))
		b.Open = ptr(tdBool(o, "is_open"))
	case "related_articles":
		b.Text = c.text(o["header"])
		b.Articles = []*richArticleJSON{}
		for _, v := range tdList(o, "articles") {
			a := tdObject(v)
			if a == nil {
				continue
			}
			ra := &richArticleJSON{URL: richSafeURL(c.str(a, "url")), Title: c.str(a, "title"),
				Description: c.str(a, "description"), Author: c.str(a, "author"),
				Date: isoPtr(tdInt(a, "publish_date"))}
			c.photo(tdObject(a["photo"]), func(m *chatMediaJSON) { ra.Photo = m })
			b.Articles = append(b.Articles, ra)
		}
	case "map":
		loc := tdObject(o["location"])
		b.Lat, b.Lon = ptr(tdFloat(loc, "latitude")), ptr(tdFloat(loc, "longitude"))
		b.Zoom = ptr(tdInt(o, "zoom"))
		b.Width, b.Height = ptr(tdInt(o, "width")), ptr(tdInt(o, "height"))
		b.Caption = c.caption(o["caption"])
	case "button_row":
		b.Buttons = []*richButtonJSON{}
		for _, v := range tdList(o, "buttons") {
			btn := tdObject(v)
			if btn == nil {
				continue
			}
			b.Buttons = append(b.Buttons, &richButtonJSON{Text: c.text(btn["text"]),
				URL: richButtonURL(btn)})
		}
		b.Align = ptr(align(o["align"], "pageBlockHorizontalAlignment", "left"))
	}
	return b
}

// addRich adds the people a rich message's texts name.
func (c *mentionCollector) addRich(d *richDoc) {
	if d == nil {
		return
	}
	c.ids = append(c.ids, d.users...)
	c.names = append(c.names, d.usernames...)
}

// richDetail is a rich message's RichDetail for the viewer: its texts'
// entities resolved through the scope and its media from m's files of
// role rich (none for an earlier version, whose m is empty). d may be
// nil (a row without content_extra).
func (v view) richDetail(k kinds, m logdb.Message, d *richDoc) richDetailJSON {
	if d == nil {
		d = parseRich(nil)
	}
	for _, t := range d.texts {
		t.Entities = make([]textEntityJSON, 0, len(t.stored))
		for _, e := range t.stored {
			j := e.textEntityJSON
			switch e.Type {
			case "mention_name":
				j.UserID = json.RawMessage("null")
				if e.userID > 0 && k.mentions.visible[e.userID] {
					j.UserID = json.RawMessage(itoa(e.userID))
				}
			case "mention":
				name := strings.ToLower(strings.TrimPrefix(spanOf16(t.Text, e.Offset, e.Length), "@"))
				if id, ok := k.mentions.owners[name]; ok && name != "" && k.mentions.visible[id] {
					j.UserID = json.RawMessage(itoa(id))
				}
			}
			t.Entities = append(t.Entities, j)
		}
	}
	for _, md := range d.media {
		if f, ok := messageFile(m, "rich", md.pos); ok {
			j := v.media(md.ctype, f, md.extra)
			// A video's or an animation's still thumbnail is the
			// file of role thumbnail at its position, its poster
			// when stored, as a main video's is (poster in
			// chats.go); a block has no cover.
			if t, ok := messageFile(m, "thumbnail", md.pos); ok && j != nil && storedFile(t) &&
				(j.Render == "video" || j.Render == "animation") {
				u := v.photo(t)
				j.Poster, j.Thumb = u, u
			}
			md.set(j)
		} else {
			md.set(nil)
		}
	}
	return d.detail
}

// spanOf16 is the part of s at UTF-16 offset off, n units long.
func spanOf16(s string, off, n int64) string {
	u := utf16.Encode([]rune(s))
	if off > int64(len(u)) || n > int64(len(u))-off {
		return ""
	}
	return string(utf16.Decode(u[off : off+n]))
}
