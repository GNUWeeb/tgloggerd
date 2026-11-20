// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package auth

import (
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20"
)

// File tokens are the opaque ids in /files/<token> URLs, made as
// web/src/auth/FileToken.cpp makes them: a deterministic authenticated
// encryption of the 64-bit file id. A 24-byte synthetic nonce, the
// keyed BLAKE2b of the little-endian id, is followed by the id
// XChaCha20-encrypted under that nonce; the 32 bytes are written as 64
// lowercase hex digits. The same id always gives the same token, and a
// token not made with the key does not decrypt.

const (
	fileTokenPlain = 8                             // the id, little-endian
	fileTokenTag   = chacha20.NonceSizeX           // 24
	fileTokenRaw   = fileTokenTag + fileTokenPlain // 32
	fileTokenHex   = fileTokenRaw * 2              // 64
)

// FileTokens encrypts and decrypts file ids.
type FileTokens struct {
	mac [32]byte // keys the synthetic nonce
	enc [32]byte // keys the stream cipher
}

// deriveSubkey is the keyed BLAKE2b-256 of a label.
func deriveSubkey(key []byte, label string) [32]byte {
	h, err := blake2b.New256(key)
	if err != nil {
		// DecodeAppKey refuses keys BLAKE2b cannot take.
		panic("auth: " + err.Error())
	}
	h.Write([]byte(label))
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// NewFileTokens derives the token subkeys from the decoded WEB_APP_KEY.
func NewFileTokens(key []byte) *FileTokens {
	return &FileTokens{
		mac: deriveSubkey(key, "tgloggerd:file-token:mac:v1"),
		enc: deriveSubkey(key, "tgloggerd:file-token:enc:v1"),
	}
}

func (f *FileTokens) tag(plain []byte) []byte {
	h, _ := blake2b.New(fileTokenTag, f.mac[:])
	h.Write(plain)
	return h.Sum(nil)
}

func (f *FileTokens) xor(dst, src, nonce []byte) {
	c, err := chacha20.NewUnauthenticatedCipher(f.enc[:], nonce)
	if err != nil {
		panic("auth: " + err.Error())
	}
	c.XORKeyStream(dst, src)
}

// Encrypt is the token for a file id.
func (f *FileTokens) Encrypt(id uint64) string {
	var plain [fileTokenPlain]byte
	binary.LittleEndian.PutUint64(plain[:], id)

	raw := make([]byte, fileTokenRaw)
	copy(raw, f.tag(plain[:]))
	f.xor(raw[fileTokenTag:], plain[:], raw[:fileTokenTag])
	return hex.EncodeToString(raw)
}

// Decrypt is the file id a token encodes. It fails on a token of the
// wrong length, one that is not hex (either case, as libsodium reads
// it), or one that was not made with the key.
func (f *FileTokens) Decrypt(tok string) (uint64, bool) {
	if len(tok) != fileTokenHex {
		return 0, false
	}
	raw, err := hex.DecodeString(tok)
	if err != nil {
		return 0, false
	}
	var plain [fileTokenPlain]byte
	f.xor(plain[:], raw[fileTokenTag:], raw[:fileTokenTag])
	if subtle.ConstantTimeCompare(f.tag(plain[:]), raw[:fileTokenTag]) != 1 {
		return 0, false
	}
	return binary.LittleEndian.Uint64(plain[:]), true
}
