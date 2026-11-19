// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"tgloggerd/webapi/internal/logdb"
)

func emptyMessage() logdb.Message { return logdb.Message{} }

// A rich message's detail (RichDetail): the page blocks converted from
// TDLib's JSON, each supported block type from a sample of its own,
// their rich text as text and entities, their media at the positions
// the daemon gives them, and over the "rich" fixture the media from
// the message's files, the people named resolved through the scope,
// an earlier version's page and a page whose JSON was over the cap.

const richLab = int64(-1001000000042)

// richSample is TDLib's JSON of a rich message whose blocks are the
// given ones, as content_extra keeps it.
func richSample(blocks ...string) []byte {
	return []byte(`{"blocks":` + fmt.Sprint(len(blocks)) + `,"is_rtl":false,"tdlib":` +
		`{"@type":"messageRichMessage","message":{"@type":"richMessage","is_rtl":true,` +
		`"is_full":true,"blocks":[` + strings.Join(blocks, ",") + `]}}}`)
}

func rtPlain(s string) string { return `{"@type":"richTextPlain","text":"` + s + `"}` }

func rtCaption(s string) string {
	return `{"@type":"pageBlockCaption","text":` + rtPlain(s) + `,"credit":` + rtPlain("") + `}`
}

func rtFile(id int) string { return fmt.Sprintf(`{"@type":"file","id":%d,"size":10}`, id) }

func rtPhoto(sizes ...string) string {
	return `{"@type":"photo","minithumbnail":{"@type":"minithumbnail","width":4,"height":3,` +
		`"data":"AAAA"},"sizes":[` + strings.Join(sizes, ",") + `]}`
}

func rtSize(w, h, file int) string {
	return fmt.Sprintf(`{"@type":"photoSize","type":"x","photo":%s,"width":%d,"height":%d}`,
		rtFile(file), w, h)
}

// richJSON is a value as the API sends it, decoded generically.
func richJSON(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// richBlocksOf converts blocks and returns the detail as sent, with no
// file known for any medium and nobody visible.
func richBlocksOf(t *testing.T, blocks ...string) (map[string]any, []any, *richDoc) {
	t.Helper()
	d := parseRich(richSample(blocks...))
	var v view
	j := richJSON(t, v.richDetail(kinds{}, emptyMessage(), d))
	return j, asList(j["blocks"]), d
}

// TestRichBlocks converts a sample of every supported block type.
func TestRichBlocks(t *testing.T) {
	j, bs, d := richBlocksOf(t,
		`{"@type":"pageBlockTitle","title":`+rtPlain("T")+`}`,
		`{"@type":"pageBlockSubtitle","subtitle":`+rtPlain("S")+`}`,
		`{"@type":"pageBlockAuthorDate","author":`+rtPlain("A")+`,"publish_date":1794927660}`,
		`{"@type":"pageBlockHeader","header":`+rtPlain("H")+`}`,
		`{"@type":"pageBlockSubheader","subheader":`+rtPlain("SH")+`}`,
		`{"@type":"pageBlockSectionHeading","text":`+rtPlain("SE")+`,"size":2}`,
		`{"@type":"pageBlockKicker","kicker":`+rtPlain("K")+`}`,
		`{"@type":"pageBlockParagraph","text":`+rtPlain("P")+`}`,
		`{"@type":"pageBlockPreformatted","text":`+rtPlain("x := 1")+`,"language":"go"}`,
		`{"@type":"pageBlockFooter","footer":`+rtPlain("F")+`}`,
		`{"@type":"pageBlockThinking","text":`+rtPlain("TH")+`}`,
		`{"@type":"pageBlockDivider"}`,
		`{"@type":"pageBlockMathematicalExpression","expression":"e^{i\\pi}"}`,
		`{"@type":"pageBlockAnchor","name":"top"}`,
		`{"@type":"pageBlockList","items":[{"@type":"pageBlockListItem","label":"1.",`+
			`"blocks":[{"@type":"pageBlockParagraph","text":`+rtPlain("one")+`}],`+
			`"has_checkbox":true,"is_checked":true}]}`,
		`{"@type":"pageBlockBlockQuote","blocks":[{"@type":"pageBlockParagraph","text":`+
			rtPlain("q")+`}],"credit":`+rtPlain("c")+`}`,
		`{"@type":"pageBlockExpandableBlockQuote","text":`+rtPlain("eq")+`,"credit":`+rtPlain("")+`}`,
		`{"@type":"pageBlockPullQuote","text":`+rtPlain("pq")+`,"credit":`+rtPlain("pc")+`}`,
		`{"@type":"pageBlockChatLink","title":"Chat","username":"chat"}`,
		`{"@type":"pageBlockTable","caption":`+rtPlain("cap")+`,"cells":[[{"@type":`+
			`"pageBlockTableCell","text":`+rtPlain("h")+`,"is_header":true,"colspan":2,"rowspan":0,`+
			`"align":{"@type":"pageBlockHorizontalAlignmentRight"},"valign":{"@type":`+
			`"pageBlockVerticalAlignmentBottom"}}],[{"@type":"pageBlockTableCell","is_header":false,`+
			`"colspan":1,"rowspan":1}]],"is_bordered":true,"is_striped":true}`,
		`{"@type":"pageBlockDetails","header":`+rtPlain("more")+`,"blocks":[{"@type":`+
			`"pageBlockDivider"}],"is_open":true}`,
		`{"@type":"pageBlockMap","location":{"@type":"location","latitude":-6.2,"longitude":`+
			`106.8},"zoom":12,"width":300,"height":200,"caption":`+rtCaption("map")+`}`,
		`{"@type":"pageBlockButtonRow","buttons":[{"@type":"inlineButton","text":`+rtPlain("Go")+
			`,"type":{"@type":"inlineKeyboardButtonTypeUrl","url":"https://example.org/"}},`+
			`{"@type":"inlineButton","text":`+rtPlain("Cb")+`,"type":{"@type":`+
			`"inlineKeyboardButtonTypeCallback","data":"eA=="}}],"align":{"@type":`+
			`"pageBlockHorizontalAlignmentCenter"}}`,
		`{"@type":"pageBlockUnsupported"}`,
		`{"@type":"pageBlockHologram","text":`+rtPlain("?")+`}`,
	)
	if j["kind"] != "rich" || j["is_rtl"] != true || j["is_full"] != true {
		t.Errorf("detail %v", j)
	}
	if len(d.media) != 0 {
		t.Errorf("media %v", d.media)
	}
	want := []string{"title", "subtitle", "author_date", "header", "subheader", "section_heading",
		"kicker", "paragraph", "preformatted", "footer", "thinking", "divider",
		"mathematical_expression", "anchor", "list", "block_quote", "expandable_block_quote",
		"pull_quote", "chat_link", "table", "details", "map", "button_row", "unknown", "unknown"}
	if len(bs) != len(want) {
		t.Fatalf("%d blocks: %v", len(bs), bs)
	}
	b := func(i int) map[string]any { return bs[i].(map[string]any) }
	text := func(v any) string { return v.(map[string]any)["text"].(string) }
	for i, w := range want {
		if b(i)["type"] != w {
			t.Errorf("block %d: %v, want %s", i, b(i), w)
		}
	}
	for i, w := range []string{"T", "S", "A", "H", "SH", "SE", "K", "P", "x := 1", "F", "TH"} {
		if text(b(i)["text"]) != w {
			t.Errorf("block %d text %v, want %s", i, b(i)["text"], w)
		}
	}
	if b(2)["date"] != iso(1794927660) || num(b(5)["size"]) != 2 || b(8)["language"] != "go" {
		t.Errorf("author date %v, heading %v, preformatted %v", b(2), b(5), b(8))
	}
	if len(b(11)) != 1 || b(12)["expression"] != `e^{i\pi}` || b(13)["name"] != "top" {
		t.Errorf("divider %v, formula %v, anchor %v", b(11), b(12), b(13))
	}
	item := asList(b(14)["items"])[0].(map[string]any)
	if item["label"] != "1." || item["checkbox"] != true || item["checked"] != true ||
		text(asList(item["blocks"])[0].(map[string]any)["text"]) != "one" {
		t.Errorf("list %v", b(14))
	}
	if text(asList(b(15)["blocks"])[0].(map[string]any)["text"]) != "q" || text(b(15)["credit"]) != "c" ||
		text(b(16)["text"]) != "eq" || text(b(17)["credit"]) != "pc" {
		t.Errorf("quotes %v %v %v", b(15), b(16), b(17))
	}
	if b(18)["title"] != "Chat" || b(18)["username"] != "chat" {
		t.Errorf("chat link %v", b(18))
	}
	rows := asList(b(19)["rows"])
	c0 := asList(rows[0])[0].(map[string]any)
	c1 := asList(rows[1])[0].(map[string]any)
	if text(b(19)["text"]) != "cap" || b(19)["bordered"] != true || b(19)["striped"] != true ||
		text(c0["text"]) != "h" || c0["header"] != true || num(c0["colspan"]) != 2 ||
		num(c0["rowspan"]) != 1 || c0["align"] != "right" || c0["valign"] != "bottom" ||
		text(c1["text"]) != "" || c1["align"] != "left" || c1["valign"] != "top" {
		t.Errorf("table %v", b(19))
	}
	if text(b(20)["text"]) != "more" || b(20)["open"] != true || len(asList(b(20)["blocks"])) != 1 {
		t.Errorf("details %v", b(20))
	}
	if b(21)["lat"] != -6.2 || b(21)["lon"] != 106.8 || num(b(21)["zoom"]) != 12 ||
		num(b(21)["width"]) != 300 || text(b(21)["caption"].(map[string]any)["text"]) != "map" {
		t.Errorf("map %v", b(21))
	}
	btns := asList(b(22)["buttons"])
	if b(22)["align"] != "center" || len(btns) != 2 ||
		btns[0].(map[string]any)["url"] != "https://example.org/" ||
		btns[1].(map[string]any)["url"] != nil || text(btns[1].(map[string]any)["text"]) != "Cb" {
		t.Errorf("buttons %v", b(22))
	}
	if b(23)["name"] != "unsupported" || b(24)["name"] != "hologram" || len(b(24)) != 2 {
		t.Errorf("unknown %v %v", b(23), b(24))
	}
}

// TestRichMediaPositions: the media blocks, each medium at the
// position the daemon numbers it, depth first and counting only media
// with a file: the photo's largest size, the embedded post's author
// photo before its own blocks, a photo without sizes not counted.
func TestRichMediaPositions(t *testing.T) {
	_, bs, d := richBlocksOf(t,
		`{"@type":"pageBlockPhoto","photo":`+rtPhoto(rtSize(10, 10, 1), rtSize(80, 60, 2),
			rtSize(80, 60, 3))+`,"caption":`+rtCaption("p")+`,"url":"https://t.me/x","has_spoiler":true}`,
		`{"@type":"pageBlockPhoto","photo":`+rtPhoto()+`,"caption":`+rtCaption("")+`,`+
			`"url":"javascript:alert(1)"}`,
		`{"@type":"pageBlockCollage","blocks":[{"@type":"pageBlockVideo","video":{"@type":"video",`+
			`"duration":5,"width":64,"height":36,"mime_type":"video/mp4","video":`+rtFile(4)+`},`+
			`"caption":`+rtCaption("")+`,"need_autoplay":true,"is_looped":true},`+
			`{"@type":"pageBlockAnimation","animation":{"@type":"animation","animation":`+rtFile(5)+
			`},"caption":`+rtCaption("")+`}],"caption":`+rtCaption("c")+`}`,
		`{"@type":"pageBlockEmbeddedPost","url":"https://example.org/p","author":"Ann",`+
			`"author_photo":`+rtPhoto(rtSize(5, 5, 6))+`,"date":0,"blocks":[{"@type":"pageBlockAudio",`+
			`"audio":{"@type":"audio","title":"Song","performer":"Band","audio":`+rtFile(7)+`},`+
			`"caption":`+rtCaption("")+`}],"caption":`+rtCaption("")+`}`,
		`{"@type":"pageBlockCover","cover":{"@type":"pageBlockDocument","document":{"@type":`+
			`"document","file_name":"a.pdf","mime_type":"application/pdf","document":`+rtFile(8)+
			`},"caption":`+rtCaption("")+`}}`,
		`{"@type":"pageBlockSlideshow","blocks":[{"@type":"pageBlockVoiceNote","voice_note":{"@type":`+
			`"voiceNote","duration":3,"waveform":"AAAA","voice":`+rtFile(9)+`},"caption":`+rtCaption("")+
			`}],"caption":`+rtCaption("")+`}`,
		`{"@type":"pageBlockEmbedded","url":"https://example.org/e","html":"<script></script>",`+
			`"poster_photo":`+rtPhoto(rtSize(1, 1, 10))+`,"width":640,"height":480,"caption":`+rtCaption("")+`}`,
		`{"@type":"pageBlockRelatedArticles","header":`+rtPlain("Related")+`,"articles":[`+
			`{"@type":"pageBlockRelatedArticle","url":"https://example.org/a","title":"A",`+
			`"description":"d","photo":`+rtPhoto(rtSize(2, 2, 11))+`,"author":"x","publish_date":0},`+
			`{"@type":"pageBlockRelatedArticle","url":"https://example.org/b","title":"B"}]}`,
		`{"@type":"pageBlockChatLink","title":"Chat","photo":{"@type":"chatPhotoInfo","small":`+
			rtFile(12)+`},"username":""}`,
	)
	type want struct {
		pos   int
		ctype string
	}
	var got []want
	for _, m := range d.media {
		got = append(got, want{m.pos, m.ctype})
	}
	exp := []want{{0, "photo"}, {1, "video"}, {2, "animation"}, {3, "photo"}, {4, "audio"},
		{5, "document"}, {6, "voice"}, {7, "photo"}, {8, "photo"}}
	if fmt.Sprint(got) != fmt.Sprint(exp) {
		t.Errorf("media %v, want %v", got, exp)
	}
	// The photo's detail is its largest size, the first of equals.
	var x contentExtra
	if err := json.Unmarshal(d.media[0].extra, &x); err != nil || x.Width != 80 || x.Height != 60 ||
		x.Minithumbnail == nil || x.Minithumbnail.Data != "AAAA" {
		t.Errorf("photo extra %s", d.media[0].extra)
	}
	b := func(i int) map[string]any { return bs[i].(map[string]any) }
	if b(0)["url"] != "https://t.me/x" || b(0)["spoiler"] != true || b(0)["type"] != "photo" {
		t.Errorf("photo %v", b(0))
	}
	if _, ok := b(1)["url"]; ok {
		t.Errorf("a javascript: link was sent: %v", b(1))
	}
	v := asList(b(2)["blocks"])[0].(map[string]any)
	if v["type"] != "video" || v["autoplay"] != true || v["loop"] != true || b(2)["type"] != "collage" {
		t.Errorf("collage %v", b(2))
	}
	if b(3)["type"] != "embedded_post" || b(3)["author"] != "Ann" || b(3)["date"] != nil ||
		b(3)["url"] != "https://example.org/p" || len(asList(b(3)["blocks"])) != 1 {
		t.Errorf("embedded post %v", b(3))
	}
	if b(4)["type"] != "cover" || asList(b(4)["blocks"])[0].(map[string]any)["type"] != "document" {
		t.Errorf("cover %v", b(4))
	}
	if b(6)["type"] != "embedded" || b(6)["url"] != "https://example.org/e" || num(b(6)["width"]) != 640 {
		t.Errorf("embedded %v", b(6))
	}
	if _, ok := b(6)["html"]; ok {
		t.Errorf("an embed's HTML was sent: %v", b(6))
	}
	arts := asList(b(7)["articles"])
	if len(arts) != 2 || arts[1].(map[string]any)["title"] != "B" {
		t.Errorf("related %v", b(7))
	}
}

// TestRichText: the formatting of a rich text as entities, offsets in
// UTF-16 code units, outer ones first.
func TestRichText(t *testing.T) {
	_, bs, d := richBlocksOf(t, `{"@type":"pageBlockParagraph","text":{"@type":"richTexts","texts":[`+
		rtPlain(`😀 `)+`,`+
		`{"@type":"richTextBold","text":{"@type":"richTextItalic","text":`+rtPlain("bi")+`}},`+
		`{"@type":"richTextUrl","text":`+rtPlain("site")+`,"url":"https://example.org/"},`+
		`{"@type":"richTextUrl","text":`+rtPlain("bad")+`,"url":"javascript:x"},`+
		`{"@type":"richTextMentionName","text":`+rtPlain("Ann")+`,"user_id":42},`+
		`{"@type":"richTextMention","text":`+rtPlain("@bob")+`,"username":"bob"},`+
		`{"@type":"richTextMention","text":`+rtPlain("Bob")+`,"username":"bob"},`+
		`{"@type":"richTextCustomEmoji","custom_emoji_id":"5368324170671202286","alternative_text":"*"},`+
		`{"@type":"richTextFixed","text":`+rtPlain("f")+`},`+
		`{"@type":"richTextSubscript","text":`+rtPlain("2")+`},`+
		`{"@type":"richTextMathematicalExpression","expression":"x"},`+
		`{"@type":"richTextIcon","width":1,"height":1},`+
		`{"@type":"richTextDiff","text":`+rtPlain("new")+`,"old_text":`+rtPlain("old")+`},`+
		`{"@type":"richTextButton","button":{"@type":"inlineButton","text":`+rtPlain("B")+
		`,"type":{"@type":"inlineKeyboardButtonTypeUrl","url":"https://example.org/b"}}}`+
		`]}}`)
	tx := bs[0].(map[string]any)["text"].(map[string]any)
	if tx["text"] != "\U0001F600 bisitebadAnn@bobBob*f2xnewB" {
		t.Errorf("text %q", tx["text"])
	}
	type e struct {
		Off, Len int64
		Type     string
		Extra    string
	}
	var got []e
	for _, v := range asList(tx["entities"]) {
		m := v.(map[string]any)
		ex := ""
		for _, k := range []string{"url", "user_id", "custom_emoji_id"} {
			if x, ok := m[k]; ok {
				ex += fmt.Sprintf("%s=%v", k, x)
			}
		}
		got = append(got, e{num(m["offset"]), num(m["length"]), m["type"].(string), ex})
	}
	want := []e{{3, 2, "bold", ""}, {3, 2, "italic", ""}, {5, 4, "text_url", "url=https://example.org/"},
		{12, 3, "mention_name", "user_id=<nil>"}, {15, 4, "mention", ""},
		{19, 3, "text_url", "url=https://t.me/bob"},
		{22, 1, "custom_emoji", "custom_emoji_id=5368324170671202286"}, {23, 1, "code", ""},
		{24, 1, "subscript", ""}, {25, 1, "code", ""}, {29, 1, "text_url", "url=https://example.org/b"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("entities\n%v, want\n%v", got, want)
	}
	if fmt.Sprint(d.users) != "[42]" || fmt.Sprint(d.usernames) != "[bob]" {
		t.Errorf("named %v %v", d.users, d.usernames)
	}
}

// TestRichNoPage: a content_extra without a page (over the cap, not
// read again yet, none at all, not JSON) has null blocks.
func TestRichNoPage(t *testing.T) {
	var v view
	for _, raw := range []string{`{"blocks":3,"is_rtl":true,"tdlib":{"@type":"messageRichMessage"},` +
		`"tdlib_omitted_bytes":300000}`, `{"tdlib":{"@type":"messageRichMessage"}}`, ``, `{`} {
		j := richJSON(t, v.richDetail(kinds{}, emptyMessage(), parseRich([]byte(raw))))
		if b, ok := j["blocks"]; !ok || b != nil || j["kind"] != "rich" || j["is_full"] != true {
			t.Errorf("%s: %v", raw, j)
		}
		if raw != "" && raw != "{" && strings.HasPrefix(raw, `{"blocks"`) && j["is_rtl"] != true {
			t.Errorf("%s: is_rtl %v", raw, j)
		}
	}
}

// TestRichDetail: over the "rich" fixture, the chat and the message
// page send the page with its media from the message's files, the
// person mentioned for a viewer that sees him only, an earlier version
// without media, and a page over the cap without blocks.
func TestRichDetail(t *testing.T) {
	e := newAdminEnv(t, "rich")
	grantMember(t, e, []int64{richLab})
	for _, role := range []string{"superadmin", "member"} {
		b := e.want(200, role, "GET", fmt.Sprintf("/api/chats/group/%d/messages?limit=10", richLab), nil)
		ms := map[int]map[string]any{}
		for _, m := range asList(b["messages"]) {
			m := m.(map[string]any)
			ms[int(num(m["id"]))] = m
		}
		d := detailOf(t, ms, 4301)
		bs := asList(d["blocks"])
		if d["kind"] != "rich" || d["is_full"] != true || len(bs) != 6 {
			t.Fatalf("%s: detail %v", role, d)
		}
		blk := func(i int) map[string]any { return bs[i].(map[string]any) }
		ents := asList(blk(1)["text"].(map[string]any)["entities"])
		mention := ents[0].(map[string]any)
		wantUser := any(nil)
		if role == "superadmin" {
			wantUser = float64(bima)
		}
		if mention["type"] != "mention_name" || mention["user_id"] != wantUser {
			t.Errorf("%s: mention %v", role, mention)
		}
		photo, _ := blk(2)["media"].(map[string]any)
		if photo == nil || photo["stored"] != true || photo["url"] == nil || num(photo["w"]) != 800 ||
			photo["render"] != "image" {
			t.Errorf("%s: photo %v", role, blk(2))
		}
		item := asList(blk(3)["items"])[0].(map[string]any)
		video, _ := asList(item["blocks"])[0].(map[string]any)["media"].(map[string]any)
		if video == nil || video["stored"] != false || video["render"] != "video" ||
			num(video["duration"]) != 12 || video["name"] != "clip.mp4" {
			t.Errorf("%s: video %v", role, item)
		}
		if blk(4)["type"] != "table" || blk(5)["type"] != "unknown" || blk(5)["name"] != "fancy_new" {
			t.Errorf("%s: %v %v", role, blk(4), blk(5))
		}
		// The earlier version: its own page, without the files.
		ed := editsOf(t, ms[4301])
		if len(ed) != 1 {
			t.Fatalf("%s: edits %v", role, ed)
		}
		vd := ed[0]["detail"].(map[string]any)
		vb := asList(vd["blocks"])
		if vd["kind"] != "rich" || vd["is_full"] != false || len(vb) != 2 ||
			vb[1].(map[string]any)["type"] != "photo" || vb[1].(map[string]any)["media"] != nil {
			t.Errorf("%s: version %v", role, vd)
		}
		// Over the cap: no blocks; the text is the page's.
		d2 := detailOf(t, ms, 4302)
		if b, ok := d2["blocks"]; !ok || b != nil || d2["is_rtl"] != true || ms[4302]["text"] != "A long page" {
			t.Errorf("%s: over the cap %v", role, ms[4302])
		}
	}

	// The message page carries the same detail.
	b := e.want(200, "superadmin", "GET", "/api/page/messages/group/4301", nil)
	msg := b["props"].(map[string]any)["message"].(map[string]any)
	pd, _ := msg["detail"].(map[string]any)
	if bs := asList(pd["blocks"]); len(bs) != 6 || bs[2].(map[string]any)["media"] == nil {
		t.Errorf("message page detail %v", msg["detail"])
	}
}
