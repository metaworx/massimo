// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package portal implements the server side of the MAX! Cube's HTTP portal
// connection: the Cube POSTs AES-encrypted protocol lines and expects
// encrypted protocol lines in return.
//
// What is known (ERNW/Insinuator 2016, verified on a real Cube in 2026):
//
//   - The Cube sends "POST /cube" with headers "Opt: \"http://www.eq-3.com/MAX\", ns=MAX"
//     and "MAX-Serial: <serial>"; the body is the raw AES-128-CBC ciphertext
//     of CR LF separated protocol lines, zero padded.
//   - Key and IV are fixed per device; the key is printed on the board.
//   - After the login request (H: + B: lines) the original portal answered
//     "i:000051b1,00000000,ffffffff" and "b:OK".
//
// Everything beyond that (poll interval, what the Cube sends while idle, how
// commands are delivered) is still to be captured.
package portal

import (
	"fmt"
	"strings"

	"go.metaworx.ch/massimo/internal/portal/portalcrypto"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Codec encrypts and decrypts portal bodies for one Cube.
type Codec struct {
	key, iv []byte
}

// NewCodec returns a Codec for the given 16-byte key and IV.
func NewCodec(key, iv []byte) (*Codec, error) {
	if len(key) != portalcrypto.KeySize || len(iv) != portalcrypto.BlockSize {
		return nil, fmt.Errorf("codec: key and iv must be %d bytes", portalcrypto.BlockSize)
	}
	return &Codec{key: append([]byte(nil), key...), iv: append([]byte(nil), iv...)}, nil
}

// Decode decrypts a request body and splits it into protocol lines. The
// plaintext (padding removed) is returned too, for logging.
func (c *Codec) Decode(body []byte) ([]cube.Message, string, error) {
	plain, err := portalcrypto.Decrypt(c.key, c.iv, body)
	if err != nil {
		return nil, "", err
	}
	text := string(portalcrypto.Unpad(plain))
	msgs, err := cube.SplitLines(text)
	return msgs, text, err
}

// Encode joins the messages with CR LF and encrypts them.
func (c *Codec) Encode(msgs []cube.Message) ([]byte, error) {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Wire())
	}
	return portalcrypto.Encrypt(c.key, c.iv, []byte(sb.String()))
}
