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
	s = strings.ToLower(strings.Join(strings.Fields(s), ""))
	s = strings.TrimPrefix(s, "k")
	k, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("key: not hex: %w", err)
	}
	if len(k) != KeySize {
		return nil, fmt.Errorf("key: want %d bytes, got %d", KeySize, len(k))
	}
	return k, nil
}

// ParseIV parses a 32-hex-digit IV.
func ParseIV(s string) ([]byte, error) {
	iv, err := hex.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		return nil, fmt.Errorf("iv: not hex: %w", err)
	}
	if len(iv) != BlockSize {
		return nil, fmt.Errorf("iv: want %d bytes, got %d", BlockSize, len(iv))
	}
	return iv, nil
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
