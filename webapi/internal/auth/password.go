// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Password hashes are the strings libsodium's crypto_pwhash_str writes
// (web/src/auth/Password.cpp), which web_users.password_hash holds:
//
//	$argon2id$v=19$m=<KiB>,t=<passes>,p=<lanes>$<salt>$<hash>
//
// with salt and hash in unpadded standard base64.

// The cost of new hashes: libsodium's OPSLIMIT_INTERACTIVE and
// MEMLIMIT_INTERACTIVE, as the Drogon web hashes, with its salt and
// hash lengths.
const (
	hashTime    = 2
	hashMemory  = 64 * 1024 // KiB
	hashThreads = 1
	hashSaltLen = 16
	hashKeyLen  = 32
)

// Limits on the hashes Verify reads, which are the argon2 reference
// decoder's, so a string libsodium refuses is refused here as well.
const (
	minSaltLen = 8
	minKeyLen  = 16
)

var phc64 = base64.RawStdEncoding.Strict()

// HashPassword hashes a password into a self-describing Argon2id
// string, as crypto_pwhash_str does with the interactive limits.
func HashPassword(password string) (string, error) {
	salt := make([]byte, hashSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, hashTime, hashMemory,
		hashThreads, hashKeyLen)
	return "$argon2id$v=19$m=" + strconv.Itoa(hashMemory) +
		",t=" + strconv.Itoa(hashTime) +
		",p=" + strconv.Itoa(hashThreads) +
		"$" + phc64.EncodeToString(salt) +
		"$" + phc64.EncodeToString(key), nil
}

type phc struct {
	id           bool // Argon2id, else Argon2i
	memory, time uint32
	threads      uint8
	salt, key    []byte
}

var errHash = errors.New("not an argon2 hash string")

func param(s, name string) (uint32, error) {
	v, ok := strings.CutPrefix(s, name+"=")
	if !ok || v == "" || v[0] < '0' || v[0] > '9' {
		return 0, errHash
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 0, errHash
	}
	return uint32(n), nil
}

// parseHash reads a hash string as crypto_pwhash_str_verify does. It
// takes the two algorithms that function takes, Argon2id and Argon2i,
// and only version 19, the one libsodium writes.
func parseHash(s string) (phc, error) {
	var h phc
	f := strings.Split(s, "$")
	if len(f) != 6 || f[0] != "" || f[2] != "v=19" {
		return h, errHash
	}
	switch f[1] {
	case "argon2id":
		h.id = true
	case "argon2i":
	default:
		return h, errHash
	}
	ps := strings.Split(f[3], ",")
	if len(ps) != 3 {
		return h, errHash
	}
	var err error
	var p uint32
	if h.memory, err = param(ps[0], "m"); err != nil {
		return h, err
	}
	if h.time, err = param(ps[1], "t"); err != nil {
		return h, err
	}
	if p, err = param(ps[2], "p"); err != nil {
		return h, err
	}
	if h.time < 1 || p < 1 || p > 255 || h.memory < 8*p {
		return h, errHash
	}
	h.threads = uint8(p)
	if h.salt, err = phc64.DecodeString(f[4]); err != nil || len(h.salt) < minSaltLen {
		return h, errHash
	}
	if h.key, err = phc64.DecodeString(f[5]); err != nil || len(h.key) < minKeyLen {
		return h, errHash
	}
	return h, nil
}

// VerifyPassword checks a password against a stored hash string. A
// malformed hash never verifies.
func VerifyPassword(hash, password string) bool {
	h, err := parseHash(hash)
	if err != nil {
		return false
	}
	derive := argon2.Key
	if h.id {
		derive = argon2.IDKey
	}
	got := derive([]byte(password), h.salt, h.time, h.memory, h.threads,
		uint32(len(h.key)))
	return subtle.ConstantTimeCompare(got, h.key) == 1
}
