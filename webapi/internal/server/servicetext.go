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
)

// The rebuilt texts of the service actions whose detail says more than
// the logger's own description (service.go): who changed the
// auto-delete timer and to how long, in words; who came near whom;
// whom users were shared, a gift or Premium given, a bot created; the
// tasks of a checklist; a suggested birthdate; a giveaway's prize and
// counts. Each is a verb phrase that follows the actor, as the stored
// description is, but giveaway_completed's, which is a sentence. A
// name the viewer may not see is never written: the line then says
// what it can without it, or stays the stored one.

// serviceTextExtra is content_extra as these texts read it, beside the
// refs serviceExtra reads.
type serviceTextExtra struct {
	Seconds    int64 `json:"seconds"`
	Distance   int64 `json:"distance"`
	MonthCount int64 `json:"month_count"`
	DayCount   int64 `json:"day_count"`
	StarCount  int64 `json:"star_count"`
	Tasks      []struct {
		ID   int64  `json:"id"`
		Text string `json:"text"`
	} `json:"tasks"`
	DoneTaskIDs    []int64 `json:"done_task_ids"`
	NotDoneTaskIDs []int64 `json:"not_done_task_ids"`
	Day            int64   `json:"day"`
	Month          int64   `json:"month"`
	Year           int64   `json:"year"`
	WinnerCount    int64   `json:"winner_count"`
	Unclaimed      int64   `json:"unclaimed_prize_count"`
	StarGiveaway   bool    `json:"star_giveaway"`
}

// autoDeletePeriod is an auto-delete timer in words, in the largest
// unit it is a whole number of: "1 day", "2 weeks", "1 month". A month
// is 30 or 31 days, as Telegram's clients offer it, a year 365 days.
func autoDeletePeriod(sec int64) string {
	const day = 86400
	switch {
	case sec%(365*day) == 0:
		return plural(sec/(365*day), "year")
	case sec%(31*day) == 0:
		return plural(sec/(31*day), "month")
	case sec%(30*day) == 0:
		return plural(sec/(30*day), "month")
	case sec%(7*day) == 0:
		return plural(sec/(7*day), "week")
	case sec%day == 0:
		return plural(sec/day, "day")
	case sec%3600 == 0:
		return plural(sec/3600, "hour")
	case sec%60 == 0:
		return plural(sec/60, "minute")
	}
	return plural(sec, "second")
}

var monthNames = [...]string{"January", "February", "March", "April", "May", "June", "July",
	"August", "September", "October", "November", "December"}

// birthdate is "14 March 1995", or "14 March" without a year; "" when
// the day or month is missing.
func birthdate(day, month, year int64) string {
	if day < 1 || day > 31 || month < 1 || month > 12 {
		return ""
	}
	s := strconv.FormatInt(day, 10) + " " + monthNames[month-1]
	if year > 0 {
		s += " " + strconv.FormatInt(year, 10)
	}
	return s
}

// tasksShown is how many tasks a line quotes before it counts the
// rest.
const tasksShown = 3

// taskList quotes the tasks that have a text and counts the others:
// `"Buy milk" and "Call Andi"`, `"Buy milk" and 2 other tasks`, or "2
// tasks" when none has a text.
func taskList(texts []string, unknown int) string {
	var names []servicePart
	for _, t := range texts {
		if len(names) == tasksShown {
			unknown++
			continue
		}
		names = append(names, servicePart{text: `"` + t + `"`})
	}
	if len(names) == 0 {
		return plural(int64(unknown), "task")
	}
	if unknown > 0 {
		names = append(names, servicePart{text: others(unknown, "task")})
	}
	return partsText(nameList(names))
}

// isActor is whether id is the message's sender, whom the line follows
// and so need not name again.
func isActor(m logdb.Message, id int64) bool {
	return id != 0 && (id == m.SenderUser || id == m.SenderChat)
}

// serviceTextParts is the text of the actions this file rebuilds (see
// above), nil where the stored text stands.
func (v view) serviceTextParts(k kinds, m logdb.Message, x *serviceExtra) []servicePart {
	var t serviceTextExtra
	if json.Unmarshal(m.Extra, &t) != nil {
		return nil
	}
	one := func(id int64) []servicePart {
		if r, ok := v.ref(k, id); ok && id != 0 {
			return []servicePart{{text: r.Title, id: id}}
		}
		return nil
	}
	party := func(p *partyRef) []servicePart {
		if p == nil {
			return nil
		}
		return one(p.id())
	}
	// fromTo names a gift's giver, when it is not the actor, and its
	// receiver, as far as the viewer may see them.
	fromTo := func(from, to int64) []servicePart {
		var out []servicePart
		if !isActor(m, from) {
			if n := one(from); n != nil {
				out = line(plain(" from "), n)
			}
		}
		if n := one(to); n != nil {
			out = line(out, plain(" to "), n)
		}
		return out
	}
	switch m.ServiceType {
	case "chat_set_message_auto_delete_time":
		p := plain("disabled auto-delete for messages")
		if t.Seconds > 0 {
			p = plain("set messages to auto-delete after " + autoDeletePeriod(t.Seconds))
		}
		// from_user_id is the user whose default timer was applied.
		if n := one(x.FromUser); n != nil {
			return line(p, plain(", the default setting of "), n)
		}
		if t.Seconds > 0 {
			return p
		}
	case "proximity_alert_triggered":
		// The traveler is the actor; one who is not is named first.
		dist := plain("came within " + plural(t.Distance, "metre"))
		watcher := party(x.Watcher)
		if x.Traveler != nil && !isActor(m, x.Traveler.id()) {
			if trav := party(x.Traveler); trav != nil {
				if watcher == nil {
					return line(plain("reported that "), trav, plain(" "), dist)
				}
				return line(plain("reported that "), trav, plain(" "), dist,
					plain(" of "), watcher)
			}
		}
		if watcher != nil {
			return line(dist, plain(" of "), watcher)
		}
	case "users_shared":
		ids := make([]int64, 0, len(x.Users))
		for _, u := range x.Users {
			ids = append(ids, u.id())
		}
		if p := v.people(k, ids, "user"); p != nil {
			return line(plain("shared "), p, plain(" with the bot"))
		}
	case "gifted_premium":
		var period string
		switch {
		case t.MonthCount > 0:
			period = " for " + plural(t.MonthCount, "month")
		case t.DayCount > 0:
			period = " for " + plural(t.DayCount, "day")
		}
		if ft := fromTo(x.Gifter, x.ReceiverUser); ft != nil || period != "" {
			return line(plain("gifted a Telegram Premium subscription"+period), ft)
		}
	case "gifted_stars":
		what := "gifted Telegram Stars"
		if t.StarCount > 0 {
			what = "gifted " + plural(t.StarCount, "Telegram Star")
		}
		if ft := fromTo(x.Gifter, x.ReceiverUser); ft != nil || t.StarCount > 0 {
			return line(plain(what), ft)
		}
	case "gift":
		var from, to int64
		if x.Sender != nil {
			from = x.Sender.id()
		}
		if x.Receiver != nil {
			to = x.Receiver.id()
		}
		if ft := fromTo(from, to); ft != nil {
			return line(plain("sent a gift"), ft)
		}
	case "managed_bot_created":
		if n := one(x.Bot); n != nil {
			return line(plain("created the bot "), n)
		}
	case "checklist_tasks_done":
		texts := map[int64]string{}
		if c := k.targets[x.ChecklistMsg]; c != nil && x.ChecklistMsg != 0 {
			for _, task := range c.Tasks {
				texts[task.ID] = task.Text
			}
		}
		list := func(ids []int64) string {
			var known []string
			unknown := 0
			for _, id := range ids {
				if s := texts[id]; s != "" {
					known = append(known, s)
				} else {
					unknown++
				}
			}
			return taskList(known, unknown)
		}
		switch done, notDone := t.DoneTaskIDs, t.NotDoneTaskIDs; {
		case len(done) > 0 && len(notDone) > 0:
			return plain("marked " + list(done) + " as done and " + list(notDone) +
				" as not done")
		case len(done) > 0:
			return plain("marked " + list(done) + " as done")
		case len(notDone) > 0:
			return plain("marked " + list(notDone) + " as not done")
		}
	case "checklist_tasks_added":
		var texts []string
		unknown := 0
		for _, task := range t.Tasks {
			if task.Text != "" {
				texts = append(texts, task.Text)
			} else {
				unknown++
			}
		}
		if len(t.Tasks) > 0 {
			return plain("added " + taskList(texts, unknown) + " to the checklist")
		}
	case "suggest_birthdate":
		if d := birthdate(t.Day, t.Month, t.Year); d != "" {
			return plain("suggested a birthdate: " + d)
		}
	case "giveaway_created":
		if t.StarCount > 0 {
			return plain("created a giveaway of " + plural(t.StarCount, "Telegram Star"))
		}
	case "giveaway_completed":
		// A sentence: the giveaway, its prize as the giveaway message
		// tells it, and the counts.
		s := "the giveaway"
		if t.StarGiveaway {
			s = "the Telegram Stars giveaway"
		}
		if g := k.targets[x.GiveawayMsg]; g != nil && x.GiveawayMsg != 0 {
			var prize string
			if p := g.Prize; p != nil {
				switch {
				case p.PremiumMonths != nil && *p.PremiumMonths > 0:
					prize = "Telegram Premium for " + plural(*p.PremiumMonths, "month")
				case p.Stars != nil && *p.Stars > 0:
					prize = plural(*p.Stars, "Telegram Star")
				}
			}
			switch {
			case prize != "" && g.PrizeDescription != "":
				s = "the giveaway of " + prize + " and " + g.PrizeDescription
			case prize != "":
				s = "the giveaway of " + prize
			case g.PrizeDescription != "":
				s += " of " + g.PrizeDescription
			}
		}
		s += " was completed"
		var counts []servicePart
		if t.WinnerCount > 0 {
			counts = append(counts, servicePart{text: plural(t.WinnerCount, "winner")})
		}
		if t.Unclaimed > 0 {
			counts = append(counts, servicePart{text: plural(t.Unclaimed, "unclaimed prize")})
		}
		if counts != nil {
			s += " with " + partsText(nameList(counts))
		}
		if s != "the giveaway was completed" {
			return plain(s)
		}
	}
	return nil
}

// loadServiceTargets reads the checklists and giveaways that the
// service actions among ms are about (k.targets), in one query.
func (s *Server) loadServiceTargets(ctx context.Context, chat int64, ms []logdb.Message,
	service map[uint64]*serviceExtra) (map[int64]*kindExtra, error) {
	var ids []int64
	for _, m := range ms {
		x := service[m.RowID]
		if x == nil {
			continue
		}
		switch m.ServiceType {
		case "checklist_tasks_done":
			ids = append(ids, x.ChecklistMsg)
		case "giveaway_completed":
			ids = append(ids, x.GiveawayMsg)
		}
	}
	out := map[int64]*kindExtra{}
	if len(ids) == 0 {
		return out, nil
	}
	raw, err := logdb.MessageExtras(ctx, s.ro, chat, ids)
	if err != nil {
		return nil, err
	}
	for id, b := range raw {
		if x := parseKindExtra(b); x != nil {
			out[id] = x
		}
	}
	return out, nil
}
