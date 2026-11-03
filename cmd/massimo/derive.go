// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"go.metaworx.ch/massimo/internal/portal/portalcrypto"
)

// deriveKey tries whether the Cube key can be computed from identifiers
// printed on the device. It needs a known e:/E: exchange of at least two
// blocks and checks every candidate with CheckCBC, which works without the
// IV. If nothing matches, the key is most likely random per device.
func deriveKey(serial, mac, rf, extra string, plain, ct []byte) error {
	if serial == "" || plain == nil || ct == nil {
		return fmt.Errorf("derive: need -serial, -plain and -cipher")
	}
	if len(ct) < 2*portalcrypto.BlockSize {
		return fmt.Errorf("derive: need a ciphertext of at least two blocks")
	}
	mac = strings.ToLower(strings.NewReplacer(":", "", "-", "", " ", "").Replace(mac))
	rf = strings.ToLower(strings.TrimSpace(rf))

	// Input strings in the spellings a firmware might plausibly use.
	inputs := map[string]string{"serial": serial}
	if mac != "" {
		inputs["mac"] = mac
		inputs["MAC"] = strings.ToUpper(mac)
		inputs["serial+mac"] = serial + mac
		inputs["mac+serial"] = mac + serial
	}
	if rf != "" {
		inputs["rf"] = rf
		inputs["RF"] = strings.ToUpper(rf)
		inputs["serial+rf"] = serial + rf
	}
	for i, x := range strings.Split(extra, ",") {
		if x = strings.TrimSpace(x); x != "" {
			inputs[fmt.Sprintf("extra%d", i)] = x
			inputs[fmt.Sprintf("extra%d lower", i)] = strings.ToLower(x)
			inputs[fmt.Sprintf("serial+extra%d", i)] = serial + x
		}
	}

	type cand struct {
		name string
		key  []byte
	}
	var cands []cand
	for name, in := range inputs {
		m := md5.Sum([]byte(in))
		s1 := sha1.Sum([]byte(in))
		s256 := sha256.Sum256([]byte(in))
		cands = append(cands,
			cand{"md5(" + name + ")", m[:]},
			cand{"sha1(" + name + ")[:16]", s1[:16]},
			cand{"sha256(" + name + ")[:16]", s256[:16]},
		)
		if raw, err := hex.DecodeString(in); err == nil && len(raw) <= 16 {
			k := make([]byte, 16)
			copy(k, raw)
			cands = append(cands, cand{"hex(" + name + ") zero-padded", k})
		}
		padded := make([]byte, 16)
		copy(padded, in)
		cands = append(cands, cand{"ascii(" + name + ") zero-padded", padded})
	}

	for _, c := range cands {
		if err := portalcrypto.CheckCBC(c.key, plain, ct); err == nil {
			fmt.Printf("MATCH: key = %s = %x\n", c.name, c.key)
			return nil
		}
	}
	fmt.Printf("no match among %d candidates; the key does not look derivable from serial/MAC/RF\n", len(cands))
	return nil
}
