// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"encoding/json"

	"tgloggerd/webapi/internal/logdb"
)

// A gift as Telegram shows it on its service line (webapi/openapi.yaml:
// ServiceGiftDetail, UpgradedGift, ServiceThemeDetail;
// docs/design/message-types.md sections 3.5, 4.2.3 and 4.3): the
// gift's sticker, or an upgraded gift's model and symbol stickers (the
// message's files of role gift at positions 0 and 1), an upgraded
// gift's model, symbol, backdrop and original details, its price, and
// who gave it to whom; the animation sticker of gifted Premium, Stars,
// grams and a giveaway prize; the upgraded gift a chat theme is made
// from. The people and chats it names go through the request scope as
// the refs do (view.refPtr): one the viewer may not see is null.

// giftRarity is how rare an upgraded gift's model, symbol or backdrop
// is: a proportion per mille, or Telegram's word for it.
type giftRarity struct {
	PerMille *int64 `json:"rarity_per_mille"`
	Rarity   string `json:"rarity"`
}

// formattedExtra is a content_extra text with its entities.
type formattedExtra struct {
	Text     string          `json:"text"`
	Entities json.RawMessage `json:"entities"`
}

// storedGift is content_extra's gift: a regular gift (id, star_count,
// emoji) or an upgraded one (the rest).
type storedGift struct {
	ID            string    `json:"id"`
	StarCount     int64     `json:"star_count"`
	Emoji         string    `json:"emoji"`
	RegularGiftID string    `json:"regular_gift_id"`
	Title         string    `json:"title"`
	Name          string    `json:"name"`
	Number        int64     `json:"number"`
	Owner         *partyRef `json:"owner"`
	Model         *struct {
		Name string `json:"name"`
		giftRarity
		Crafted bool `json:"crafted"`
	} `json:"model"`
	Symbol *struct {
		Name string `json:"name"`
		giftRarity
	} `json:"symbol"`
	Backdrop *struct {
		Name        string `json:"name"`
		CenterColor int64  `json:"center_color"`
		EdgeColor   int64  `json:"edge_color"`
		SymbolColor int64  `json:"symbol_color"`
		TextColor   int64  `json:"text_color"`
		giftRarity
	} `json:"backdrop"`
	Original *struct {
		Sender   *partyRef       `json:"sender"`
		Receiver *partyRef       `json:"receiver"`
		Text     *formattedExtra `json:"text"`
		Date     int64           `json:"date"`
	} `json:"original_details"`
}

// giftExtra is content_extra as a gift's detail reads it, and a chat
// theme's.
type giftExtra struct {
	Gift         *storedGift     `json:"gift"`
	Sender       *partyRef       `json:"sender"`
	Receiver     *partyRef       `json:"receiver"`
	Gifter       int64           `json:"gifter_user_id"`
	ReceiverUser int64           `json:"receiver_user_id"`
	StarCount    int64           `json:"star_count"`
	Price        *giftPriceJSON  `json:"price"`
	Text         *formattedExtra `json:"text"`
	ThemeName    string          `json:"theme_name"`
	GiftID       string          `json:"gift_id"`
	GiftName     string          `json:"gift_name"`
}

// giftPriceJSON is a price in Stars or in hundredths of a gram.
type giftPriceJSON struct {
	Stars     *int64 `json:"stars,omitempty"`
	GramCents *int64 `json:"gram_cents,omitempty"`
}

// regularGiftTypes are the service types of a regular gift,
// upgradedGiftTypes those of an upgraded one, and giftStickerTypes
// those that carry the animation sticker of what was given.
var (
	regularGiftTypes  = map[string]bool{"gift": true, "refunded_upgraded_gift": true}
	upgradedGiftTypes = map[string]bool{"upgraded_gift": true,
		"upgraded_gift_purchase_offer": true, "upgraded_gift_purchase_offer_rejected": true}
	giftStickerTypes = map[string]bool{"gifted_premium": true, "gifted_stars": true,
		"gifted_grams": true, "giveaway_prize_stars": true}
)

// isGiftType is whether a service type has a gift detail.
func isGiftType(t string) bool {
	return regularGiftTypes[t] || upgradedGiftTypes[t] || giftStickerTypes[t]
}

// addGift records the gift or the chat theme of a service message of
// the page, with the users and chats it names beyond its refs: an
// upgraded gift's owner and original sender and receiver.
func (k *kinds) addGift(m logdb.Message, users, chats *[]int64) {
	if !isGiftType(m.ServiceType) && m.ServiceType != "chat_set_theme" {
		return
	}
	var x giftExtra
	if len(m.Extra) == 0 || json.Unmarshal(m.Extra, &x) != nil {
		// The stickers are files of the message, which a row
		// without its detail may have all the same.
		x = giftExtra{}
	}
	if k.gifts == nil {
		k.gifts = map[uint64]*giftExtra{}
	}
	k.gifts[m.RowID] = &x
	add := func(p *partyRef) {
		if p == nil {
			return
		}
		switch id := p.id(); {
		case id > 0:
			*users = append(*users, id)
		case id < 0:
			*chats = append(*chats, id)
		}
	}
	if g := x.Gift; g != nil && upgradedGiftTypes[m.ServiceType] {
		add(g.Owner)
		if o := g.Original; o != nil {
			add(o.Sender)
			add(o.Receiver)
		}
	}
}

// addGiftTexts adds the people and custom emoji a gift's texts (its
// own, and an upgraded gift's original one) mention to mc.
func (k *kinds) addGiftTexts(mc *mentionCollector, row uint64) {
	x := k.gifts[row]
	if x == nil {
		return
	}
	if t := x.Text; t != nil {
		mc.add(t.Text, t.Entities, "gift text")
	}
	if g := x.Gift; g != nil && g.Original != nil && g.Original.Text != nil {
		mc.add(g.Original.Text.Text, g.Original.Text.Entities, "gift text")
	}
}

type (
	serviceGiftJSON struct {
		Kind          string            `json:"kind"`
		Sticker       *chatMediaJSON    `json:"sticker"`
		SymbolSticker *chatMediaJSON    `json:"symbol_sticker"`
		StarCount     *int64            `json:"star_count"`
		Price         *giftPriceJSON    `json:"price"`
		Sender        *chatRefJSON      `json:"sender"`
		Receiver      *chatRefJSON      `json:"receiver"`
		Text          string            `json:"text"`
		Entities      []textEntityJSON  `json:"entities"`
		Upgraded      *upgradedGiftJSON `json:"upgraded"`
	}
	upgradedGiftJSON struct {
		ID            string            `json:"id"`
		RegularGiftID *string           `json:"regular_gift_id"`
		Title         string            `json:"title"`
		Name          string            `json:"name"`
		Number        int64             `json:"number"`
		Owner         *chatRefJSON      `json:"owner"`
		Model         *giftModelJSON    `json:"model"`
		Symbol        *giftSymbolJSON   `json:"symbol"`
		Backdrop      *giftBackdropJSON `json:"backdrop"`
		Original      *giftOriginalJSON `json:"original_details"`
	}
	giftRarityJSON struct {
		PerMille *int64  `json:"rarity_per_mille"`
		Rarity   *string `json:"rarity"`
	}
	giftModelJSON struct {
		Name string `json:"name"`
		giftRarityJSON
		Crafted bool `json:"crafted"`
	}
	giftSymbolJSON struct {
		Name string `json:"name"`
		giftRarityJSON
	}
	giftBackdropJSON struct {
		Name        string `json:"name"`
		CenterColor int64  `json:"center_color"`
		EdgeColor   int64  `json:"edge_color"`
		SymbolColor int64  `json:"symbol_color"`
		TextColor   int64  `json:"text_color"`
		giftRarityJSON
	}
	giftOriginalJSON struct {
		Sender   *chatRefJSON     `json:"sender"`
		Receiver *chatRefJSON     `json:"receiver"`
		Text     string           `json:"text"`
		Entities []textEntityJSON `json:"entities"`
		Date     *string          `json:"date"`
	}
	serviceThemeJSON struct {
		Kind      string  `json:"kind"`
		ThemeName *string `json:"theme_name"`
		GiftID    *string `json:"gift_id"`
		GiftName  *string `json:"gift_name"`
	}
)

// json is a rarity as the API gives it: a proportion, else a word
// it knows.
func (r giftRarity) json() giftRarityJSON {
	j := giftRarityJSON{PerMille: r.PerMille}
	switch r.Rarity {
	case "uncommon", "rare", "epic", "legendary":
		if j.PerMille == nil {
			s := r.Rarity
			j.Rarity = &s
		}
	}
	return j
}

// partyPtr is a party as the viewer may see it; nil for none.
func (v view) partyPtr(k kinds, p *partyRef) *chatRefJSON {
	if p == nil || p.id() == 0 {
		return nil
	}
	return v.refPtr(k, p.id())
}

// userPtr is a user as the viewer may see it; nil for none.
func (v view) userPtr(k kinds, id int64) *chatRefJSON {
	if id == 0 {
		return nil
	}
	return v.refPtr(k, id)
}

// giftText is a stored text and its entities, as the API gives them.
func (v view) giftText(k kinds, t *formattedExtra) (string, []textEntityJSON) {
	if t == nil || t.Text == "" {
		return "", []textEntityJSON{}
	}
	return t.Text, v.entities(k, t.Text, t.Entities, "gift text")
}

// nonZero is a pointer to n, nil for 0 (a count the daemon did not
// store).
func nonZero(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

// upgradedGift is an upgraded gift as the API gives it.
func (v view) upgradedGift(k kinds, g *storedGift) *upgradedGiftJSON {
	j := &upgradedGiftJSON{ID: g.ID, Title: g.Title, Name: g.Name, Number: g.Number,
		Owner: v.partyPtr(k, g.Owner)}
	if g.RegularGiftID != "" {
		id := g.RegularGiftID
		j.RegularGiftID = &id
	}
	if m := g.Model; m != nil {
		j.Model = &giftModelJSON{Name: m.Name, giftRarityJSON: m.giftRarity.json(), Crafted: m.Crafted}
	}
	if s := g.Symbol; s != nil {
		j.Symbol = &giftSymbolJSON{Name: s.Name, giftRarityJSON: s.giftRarity.json()}
	}
	if b := g.Backdrop; b != nil {
		j.Backdrop = &giftBackdropJSON{Name: b.Name, CenterColor: b.CenterColor,
			EdgeColor: b.EdgeColor, SymbolColor: b.SymbolColor, TextColor: b.TextColor,
			giftRarityJSON: b.giftRarity.json()}
	}
	if o := g.Original; o != nil {
		oj := &giftOriginalJSON{Sender: v.partyPtr(k, o.Sender),
			Receiver: v.partyPtr(k, o.Receiver), Date: isoPtr(o.Date)}
		oj.Text, oj.Entities = v.giftText(k, o.Text)
		j.Original = oj
	}
	return j
}

// giftSticker is the message's gift sticker at pos, as media.
func (v view) giftSticker(m logdb.Message, pos int, emoji string) *chatMediaJSON {
	f, ok := messageFile(m, "gift", pos)
	if !ok {
		return nil
	}
	var extra []byte
	if emoji != "" {
		extra, _ = json.Marshal(map[string]string{"emoji": emoji})
	}
	return v.media("sticker", f, extra)
}

// giftDetail is a gift service message's ServiceGiftDetail; nil when
// nothing of it is stored.
func (v view) giftDetail(k kinds, m logdb.Message) any {
	x := k.gifts[m.RowID]
	if x == nil {
		return nil
	}
	t := m.ServiceType
	j := serviceGiftJSON{Kind: "gift"}
	var emoji string
	switch {
	case regularGiftTypes[t]:
		if x.Gift != nil {
			j.StarCount, emoji = nonZero(x.Gift.StarCount), x.Gift.Emoji
		}
		j.Sender, j.Receiver = v.partyPtr(k, x.Sender), v.partyPtr(k, x.Receiver)
	case upgradedGiftTypes[t]:
		if x.Gift != nil {
			j.Upgraded = v.upgradedGift(k, x.Gift)
		}
		j.Sender, j.Receiver = v.partyPtr(k, x.Sender), v.partyPtr(k, x.Receiver)
		if p := x.Price; p != nil && (p.Stars != nil || p.GramCents != nil) {
			j.Price = p
		}
	case t == "gifted_stars", t == "giveaway_prize_stars":
		j.StarCount = nonZero(x.StarCount)
		fallthrough
	default:
		j.Sender, j.Receiver = v.userPtr(k, x.Gifter), v.userPtr(k, x.ReceiverUser)
	}
	j.Sticker = v.giftSticker(m, 0, emoji)
	if upgradedGiftTypes[t] {
		j.SymbolSticker = v.giftSticker(m, 1, "")
	}
	j.Text, j.Entities = v.giftText(k, x.Text)
	if j.Sticker == nil && j.SymbolSticker == nil && j.StarCount == nil && j.Price == nil &&
		j.Sender == nil && j.Receiver == nil && j.Text == "" && j.Upgraded == nil {
		return nil
	}
	return j
}

// themeDetail is chat_set_theme's ServiceThemeDetail; nil when no
// theme is stored.
func (v view) themeDetail(k kinds, m logdb.Message) any {
	x := k.gifts[m.RowID]
	if x == nil {
		return nil
	}
	str := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	j := serviceThemeJSON{Kind: "theme", ThemeName: str(x.ThemeName), GiftID: str(x.GiftID),
		GiftName: str(x.GiftName)}
	if j.ThemeName == nil && j.GiftID == nil && j.GiftName == nil {
		return nil
	}
	return j
}
