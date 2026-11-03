// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package portalcrypto implements the encryption the MAX! Cube uses for its
// HTTP portal connection: AES-128 in CBC mode with a per-device key and a
// fixed IV, zero padding, base64 transport. The Cube's local e:/d:
// commands use the same primitive, which makes it possible to verify this
// implementation against a real device without knowing anything about the
// portal itself.
//
// Known from the ERNW/Insinuator analysis (2016): the key is printed as a QR
// code ("k" + 32 hex digits) on the Cube's circuit board; the key size is
// 128 bits; CBC was inferred from the e: command's behaviour; the IV never
// crosses the wire.
package portalcrypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// BlockSize is the AES block size.
const BlockSize = aes.BlockSize

// KeySize is the AES-128 key size.
const KeySize = 16

// ParseKey parses the key as printed in the QR code: 32 hex digits with an
// optional leading "k". Whitespace is ignored.
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "\ufeff") // UTF-8 byte order mark from some editors
	s = strings.ToLower(strings.Join(strings.Fields(s), ""))
	s = strings.TrimPrefix(s, "k")
	return parseHex("key", s, KeySize)
}

// parseHex decodes exactly n bytes of hex and describes problems without
// echoing the value, since it may be a secret.
func parseHex(what, s string, n int) ([]byte, error) {
	for i, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, fmt.Errorf("%s: character %d of %d is not a hex digit (got %q)", what, i+1, len(s), c)
		}
	}
	if len(s) != 2*n {
		return nil, fmt.Errorf("%s: want %d hex digits, got %d", what, 2*n, len(s))
	}
	return hex.DecodeString(s)
}

// ParseIV parses a 32-hex-digit IV.
func ParseIV(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "\ufeff")
	return parseHex("iv", strings.ToLower(strings.Join(strings.Fields(s), "")), BlockSize)
}

// Pad appends zero bytes up to the next block boundary. Data that is already
// block-aligned is returned unchanged, as observed on the Cube: encrypting 16
// bytes yields 16 bytes.
func Pad(p []byte) []byte {
	if r := len(p) % BlockSize; r != 0 {
		p = append(append([]byte(nil), p...), make([]byte, BlockSize-r)...)
	}
	return p
}

// Unpad removes trailing zero bytes.
func Unpad(p []byte) []byte { return bytes.TrimRight(p, "\x00") }

// Encrypt encrypts plain (zero-padded) with AES-128-CBC.
func Encrypt(key, iv, plain []byte) ([]byte, error) {
	c, err := newCBC(key, iv, true)
	if err != nil {
		return nil, err
	}
	p := Pad(plain)
	out := make([]byte, len(p))
	c.CryptBlocks(out, p)
	return out, nil
}

// Decrypt decrypts ct with AES-128-CBC. Padding is not removed; use Unpad.
func Decrypt(key, iv, ct []byte) ([]byte, error) {
	if len(ct)%BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of %d", len(ct), BlockSize)
	}
	c, err := newCBC(key, iv, false)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(ct))
	c.CryptBlocks(out, ct)
	return out, nil
}

// RecoverIV computes the IV from the first ciphertext block and the known
// first plaintext block: in CBC, C1 = E(P1 xor IV), so IV = D(C1) xor P1.
func RecoverIV(key, firstCipherBlock, firstPlainBlock []byte) ([]byte, error) {
	if len(firstCipherBlock) < BlockSize || len(firstPlainBlock) < BlockSize {
		return nil, errors.New("need at least one full block of ciphertext and plaintext")
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, BlockSize)
	b.Decrypt(iv, firstCipherBlock[:BlockSize])
	for i := range iv {
		iv[i] ^= firstPlainBlock[i]
	}
	return iv, nil
}

// CheckCBC verifies that ct is the CBC encryption of plain under key with
// any IV, by checking every block after the first. It needs at least two
// blocks and is what distinguishes CBC from ECB and from a per-message IV.
func CheckCBC(key, plain, ct []byte) error {
	p := Pad(plain)
	if len(p) != len(ct) {
		return fmt.Errorf("length mismatch: %d plaintext vs %d ciphertext bytes", len(p), len(ct))
	}
	if len(ct) < 2*BlockSize {
		return errors.New("need at least two blocks to check chaining")
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	tmp := make([]byte, BlockSize)
	for off := BlockSize; off < len(ct); off += BlockSize {
		b.Decrypt(tmp, ct[off:off+BlockSize])
		for i := range tmp {
			tmp[i] ^= ct[off-BlockSize+i]
		}
		if !bytes.Equal(tmp, p[off:off+BlockSize]) {
			return fmt.Errorf("block %d does not match CBC with this key", off/BlockSize+1)
		}
	}
	return nil
}

func newCBC(key, iv []byte, encrypt bool) (cipher.BlockMode, error) {
	if len(iv) != BlockSize {
		return nil, fmt.Errorf("iv: want %d bytes, got %d", BlockSize, len(iv))
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if encrypt {
		return cipher.NewCBCEncrypter(b, iv), nil
	}
	return cipher.NewCBCDecrypter(b, iv), nil
}
