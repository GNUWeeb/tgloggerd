// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package mcp

// Corpus noise, as opposed to the grammatical stopwords in
// stopwords.go: the Drogon web's NoiseWords.hpp, word for word, so both
// services filter the same words.
//
// The vendored Indonesian and English stopword lists are built from
// formal prose, and formal prose is not what a Telegram group produces.
// They contain "tidak" but not "gak", "yang" but not "yg", and nothing
// at all for "wkwk", "lol" or the Date:/Subject: lines of quoted mail.
// Left alone, the top of a ranking fills with those instead of with
// what the group talks about.
//
// A separate, hand-maintained list rather than an addition to the
// generated stopwords, which regenerating would silently drop. Grouped
// by why each word is noise, because the judgement is what a future
// reader will want to argue with.
//
// The line to hold is TOPICAL vs NOT. Words that could name a subject
// stay out of this list even when they are frequent -- "net", "io",
// "id" and "co" are omitted for exactly that reason: they read as noise
// in a chat corpus and as kernel subsystems in this one. Callers who
// disagree with a specific call have telegram_popular_words's `exclude`
// and `include_stopwords` arguments.
var noiseWords = map[string]struct{}{
	// Indonesian negation, affirmation and their chat spellings.
	"bener": {}, "beneran": {}, "bukannya": {}, "emang": {},
	"emangnya": {}, "emg": {}, "enggak": {}, "ga": {}, "gak": {},
	"gakpapa": {}, "gapapa": {}, "gk": {}, "gpp": {}, "iya": {},
	"iyaa": {}, "iyah": {}, "iye": {}, "kaga": {}, "kagak": {},
	"ngak": {}, "ngga": {}, "nggak": {}, "oke": {}, "okelah": {},
	"okey": {}, "siap": {}, "sip": {}, "udahlah": {}, "yaa": {},
	"yah": {}, "yasudah": {}, "yaudah": {}, "yoi": {}, "yup": {},

	// Indonesian discourse particles -- they attach to any sentence
	// and mean nothing alone.
	"aja": {}, "ajah": {}, "dah": {}, "deh": {}, "doang": {},
	"doank": {}, "dong": {}, "donk": {}, "ealah": {}, "kah": {},
	"kan": {}, "kek": {}, "kok": {}, "lah": {}, "lho": {},
	"loh": {}, "mah": {}, "nih": {}, "ny": {}, "nya": {}, "sih": {},
	"toh": {}, "tuh": {}, "ya": {}, "yaelah": {}, "yalah": {},

	// Indonesian colloquial pronouns and address forms.
	"abang": {}, "ane": {}, "bang": {}, "bro": {}, "bund": {},
	"bunda": {}, "doi": {}, "elo": {}, "elu": {}, "ente": {},
	"gan": {}, "gua": {}, "gue": {}, "gw": {}, "kak": {},
	"kakak": {}, "lo": {}, "lu": {}, "mas": {}, "mba": {},
	"mbak": {}, "mimin": {}, "min": {}, "om": {}, "sis": {},
	"tante": {},

	// Indonesian colloquial spellings of words the formal list
	// already covers.
	"abis": {}, "apaan": {}, "banget": {}, "belom": {},
	"bentar": {}, "bgt": {}, "bikin": {}, "blm": {}, "bngt": {},
	"bntr": {}, "dgn": {}, "dgr": {}, "dikit": {}, "dll": {},
	"dmn": {}, "drpd": {}, "dsb": {}, "dst": {}, "gimana": {},
	"gini": {}, "gitu": {}, "gmn": {}, "gmna": {}, "gt": {},
	"intinya": {}, "jd": {}, "jg": {}, "kalo": {}, "karna": {},
	"kaya": {}, "kayak": {}, "kayanya": {}, "klo": {}, "klu": {},
	"knapa": {}, "knp": {}, "krn": {}, "liat": {}, "makanya": {},
	"mirip": {}, "ngapain": {}, "ngerti": {}, "ntar": {},
	"paham": {}, "pake": {}, "pokoknya": {}, "sebenernya": {},
	"skrg": {}, "soalnya": {}, "spt": {}, "sy": {}, "tadi": {},
	"tau": {}, "tdk": {}, "tp": {}, "tpi": {}, "trs": {},
	"trus": {}, "tsb": {}, "udah": {}, "udh": {}, "utk": {},
	"yg": {},

	// Interjections and evaluations -- reaction, not subject.
	"aduh": {}, "anjay": {}, "anjir": {}, "anjrit": {},
	"astaga": {}, "awok": {}, "awokawok": {}, "buset": {},
	"ehh": {}, "gokil": {}, "hadeh": {}, "haduh": {}, "haha": {},
	"hehe": {}, "hihi": {}, "hoho": {}, "keren": {}, "mantap": {},
	"mantul": {}, "masyaallah": {}, "njir": {}, "ohh": {},
	"parah": {}, "serius": {}, "waduh": {}, "wah": {}, "weh": {},
	"wih": {}, "wkwk": {}, "wkwkwk": {}, "xixi": {},

	// English internet shorthand.
	"afaik": {}, "afk": {}, "aka": {}, "asap": {}, "atm": {},
	"brb": {}, "btw": {}, "etc": {}, "ez": {}, "fyi": {}, "gg": {},
	"gj": {}, "gotcha": {}, "idc": {}, "idk": {}, "iirc": {},
	"ikr": {}, "ily": {}, "imho": {}, "imma": {}, "imo": {},
	"irl": {}, "lel": {}, "lmao": {}, "lmfao": {}, "lmk": {},
	"lol": {}, "ngl": {}, "nvm": {}, "omfg": {}, "omg": {},
	"rofl": {}, "smh": {}, "tbh": {}, "tldr": {}, "ttyl": {},
	"wdym": {}, "wtf": {}, "wth": {}, "xd": {}, "xdd": {},

	// English chat fillers, backchannels and interjections.
	"aah": {}, "actually": {}, "ahh": {}, "aight": {}, "aint": {},
	"alright": {}, "anyway": {}, "anyways": {}, "basically": {},
	"bruh": {}, "coz": {}, "crap": {}, "cuz": {}, "damn": {},
	"dang": {}, "dude": {}, "dunno": {}, "fck": {}, "fuck": {},
	"fucking": {}, "gimme": {}, "gonna": {}, "gotta": {},
	"guys": {}, "hello": {}, "hey": {}, "hmm": {}, "huh": {},
	"kay": {}, "kinda": {}, "lemme": {}, "like": {},
	"literally": {}, "mate": {}, "meh": {}, "nah": {}, "nop": {},
	"nope": {}, "okay": {}, "okie": {}, "ooh": {}, "oops": {},
	"pls": {}, "plz": {}, "probs": {}, "prolly": {}, "really": {},
	"rly": {}, "seriously": {}, "shit": {}, "sorry": {},
	"sorta": {}, "srsly": {}, "sure": {}, "thank": {}, "thanks": {},
	"tho": {}, "thru": {}, "thx": {}, "tks": {}, "tysm": {},
	"ugh": {}, "uhh": {}, "umm": {}, "wanna": {}, "well": {},
	"wow": {}, "yall": {}, "yea": {}, "yeah": {}, "yep": {},

	// Quoted-mail and forwarded-message scaffolding.
	"bcc": {}, "cc": {}, "cheers": {}, "date": {}, "dear": {},
	"forwarded": {}, "fw": {}, "fwd": {}, "regards": {},
	"replied": {}, "reply": {}, "sent": {}, "sincerely": {},
	"subject": {}, "unsubscribe": {}, "wrote": {},

	// Calendar vocabulary -- when something happened, never what it
	// was about.
	"apr": {}, "april": {}, "aug": {}, "august": {}, "bulan": {},
	"dec": {}, "december": {}, "detik": {}, "feb": {},
	"february": {}, "fri": {}, "friday": {}, "gmt": {}, "hari": {},
	"jam": {}, "jan": {}, "january": {}, "jul": {}, "july": {},
	"jumat": {}, "jun": {}, "june": {}, "kamis": {}, "malam": {},
	"mar": {}, "march": {}, "may": {}, "menit": {}, "minggu": {},
	"mon": {}, "monday": {}, "nov": {}, "november": {}, "oct": {},
	"october": {}, "pagi": {}, "rabu": {}, "sabtu": {}, "sat": {},
	"saturday": {}, "selasa": {}, "senin": {}, "sep": {},
	"sept": {}, "september": {}, "siang": {}, "sore": {}, "sun": {},
	"sunday": {}, "tahun": {}, "thu": {}, "thur": {}, "thurs": {},
	"thursday": {}, "tue": {}, "tues": {}, "tuesday": {}, "utc": {},
	"wed": {}, "wednesday": {}, "wib": {}, "wita": {},

	// Leftovers of pasted links and markup that survive URL
	// stripping.
	"com": {}, "gif": {}, "htm": {}, "html": {}, "http": {},
	"https": {}, "jpeg": {}, "jpg": {}, "org": {}, "pdf": {},
	"php": {}, "png": {}, "svg": {}, "www": {},
}

// isNoiseWord reports whether w (already lowercased and
// elongation-collapsed) is noise.
func isNoiseWord(w string) bool {
	_, ok := noiseWords[w]
	return ok
}
