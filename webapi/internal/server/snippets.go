// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
	"strconv"
	"strings"

	"tgloggerd/webapi/internal/logdb"
	"tgloggerd/webapi/internal/scope"
)

// A message without text is summarised by its kind wherever a line of
// text stands for it (a reply's quote, a message list's snippet, the
// dashboard's deleted messages): a poll by its question, a venue by its
// title, a file by its name. The summary is made of the kind's detail
// (kinds.go), so a row logged before daemon migration 000034, which
// has none, and a kind without a summary keep the "[<kind>]"
// placeholder. Names in it go through the request scope as the
// detail's do: a story's poster the viewer may not see is not named.

// addSummarised records the detail of a message the page summarises
// without showing it (a reply's target), with the polls and the
// parties its summary reads.
func (k *kinds) addSummarised(row uint64, ctype string, extra []byte, users, chats, polls *[]int64) {
	if !detailKinds[ctype] || k.extra[row] != nil {
		return
	}
	x := parseKindExtra(extra)
	if x == nil {
		return
	}
	k.extra[row] = x
	switch ctype {
	case "poll":
		*polls = append(*polls, x.pollID())
	case "story":
		switch id := x.PosterChatID; {
		case id > 0:
			*users = append(*users, id)
		case id < 0:
			*chats = append(*chats, id)
		}
	}
}

// loadSummaries loads what the summaries of the messages without text
// among ms read, as sc may see it: their detail, their polls and their
// stories' posters. Unlike loadKinds it loads nothing for the text.
func (s *Server) loadSummaries(ctx context.Context, sc scope.Scope, ms []logdb.Message) (kinds, error) {
	k := kinds{extra: map[uint64]*kindExtra{}, service: map[uint64]*serviceExtra{}}
	var users, chats, polls []int64
	for _, m := range ms {
		if m.Text == "" {
			k.addSummarised(m.RowID, m.ContentType, m.Extra, &users, &chats, &polls)
		}
	}
	var err error
	if k.polls, err = logdb.PollsByID(ctx, s.ro, polls); err != nil {
		return k, err
	}
	k.users, k.groups, err = logdb.Refs(ctx, s.ro, sc, 0, users, chats)
	return k, err
}

// callTitle names a call by its kind.
func callTitle(x *kindExtra) string {
	switch {
	case x.Group && x.Video:
		return "Video chat"
	case x.Group:
		return "Voice chat"
	case x.Video:
		return "Video call"
	}
	return "Voice call"
}

// callOutcomeText words callOutcome for a summary.
var callOutcomeText = map[string]string{"missed": "missed", "declined": "declined",
	"disconnected": "disconnected", "hung_up": "ended", "upgraded": "moved to a group call",
	"active": "ongoing", "ended": "ended"}

// duration writes seconds as m:ss, or h:mm:ss from an hour.
func duration(sec int64) string {
	h, m, s := sec/3600, sec/60%60, sec%60
	two := func(n int64) string {
		if n < 10 {
			return "0" + strconv.FormatInt(n, 10)
		}
		return strconv.FormatInt(n, 10)
	}
	if h > 0 {
		return strconv.FormatInt(h, 10) + ":" + two(m) + ":" + two(s)
	}
	return strconv.FormatInt(m, 10) + ":" + two(s)
}

// plural is n with its noun, in the singular for one.
func plural(n int64, noun string) string {
	s := strconv.FormatInt(n, 10) + " " + noun
	if n != 1 {
		s += "s"
	}
	return s
}

// summary is a message without text as one line: its kind's detail
// summarised, else its file's name, else "[<kind>]". row keys its
// detail in k.
func (v view) summary(k kinds, row uint64, ctype, fileName string) string {
	if s := v.kindSummary(k, row, ctype); s != "" {
		return s
	}
	if fileName != "" {
		return fileName
	}
	return "[" + ctype + "]"
}

// kindSummary is the summary of the kinds that have one, "" for the
// others and for a row without its detail.
func (v view) kindSummary(k kinds, row uint64, ctype string) string {
	x := k.extra[row]
	if x == nil {
		return ""
	}
	switch ctype {
	case "poll":
		if p := k.polls[x.pollID()]; p != nil {
			return p.Question
		}
	case "venue":
		return x.Title
	case "location":
		if x.Live {
			return "Live location"
		}
		return "Location"
	case "contact":
		return strings.TrimSpace(x.FirstName + " " + x.LastName)
	case "call":
		s := callTitle(x) + ", " + callOutcomeText[callOutcome(x)]
		if x.Duration > 0 {
			s += " (" + duration(x.Duration) + ")"
		}
		return s
	case "dice":
		emoji := x.Emoji
		if emoji == "" {
			// TDLib's default die.
			emoji = "\U0001F3B2"
		}
		if x.Value > 0 {
			return emoji + " " + strconv.FormatInt(x.Value, 10)
		}
		return emoji
	case "game", "invoice":
		return x.Title
	case "story":
		if r, ok := v.ref(k, x.PosterChatID); ok && x.PosterChatID != 0 {
			return "Story by " + r.Title
		}
		return "Story"
	case "giveaway":
		// The prize, and what the giveaway adds to it.
		var prizes []string
		if p := x.Prize; p != nil && p.PremiumMonths != nil {
			prizes = append(prizes, "Telegram Premium for "+plural(*p.PremiumMonths, "month"))
		} else if p != nil && p.Stars != nil {
			prizes = append(prizes, plural(*p.Stars, "Star"))
		}
		if x.PrizeDescription != "" {
			prizes = append(prizes, x.PrizeDescription)
		}
		if len(prizes) == 0 {
			return "Giveaway"
		}
		return "Giveaway: " + strings.Join(prizes, " and ")
	}
	return ""
}

// snippetOf is a message's text for a one-line quote, or its summary
// when it has none.
func (v view) snippetOf(k kinds, text string, row uint64, ctype, fileName string) string {
	if text == "" && ctype != "" && ctype != "text" {
		return v.summary(k, row, ctype, fileName)
	}
	return text
}
