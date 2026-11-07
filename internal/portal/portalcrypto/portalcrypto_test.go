// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package portalcrypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func randBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseKey(t *testing.T) {
	want := "98bbce3f1b25df8e6894b779456d330e"
	for _, in := range []string{want, "k" + want, "K98BBCE3F 1B25DF8E 6894B779 456D330E"} {
		k, err := ParseKey(in)
		if err != nil || hex.EncodeToString(k) != want {
			t.Errorf("%q: got %x, %v", in, k, err)
		}
	}
	for in, msg := range map[string]string{
		"":              "want 32 hex digits, got 0",
		"K" + want[:31]: "want 32 hex digits, got 31",
		"zz" + want[2:]: "character 1 of 32 is not a hex digit",
		"\ufeff" + want: "",
	} {
		_, err := ParseKey(in)
		if msg == "" {
			if err != nil {
				t.Errorf("%q: unexpected error %v", in, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%q: got %v, want %q", in, err, msg)
		}
	}
}

func TestPad(t *testing.T) {
	if got := Pad([]byte("TEST")); len(got) != 16 || got[4] != 0 {
		t.Errorf("got %x", got)
	}
	full := bytes.Repeat([]byte{0xff}, 16)
	if got := Pad(full); len(got) != 16 {
		t.Errorf("block-aligned input must not grow: %d", len(got))
	}
	if got := Unpad(Pad([]byte("TEST"))); string(got) != "TEST" {
		t.Errorf("unpad: %q", got)
	}
}

func TestRoundTripAndIVRecovery(t *testing.T) {
	key, iv := randBytes(t, 16), randBytes(t, 16)
	plain := []byte("H:KMD0000505,0a0106,0113\r\nB:FOOBAR1,5a8a4a5d3c1bd612b8bf1e2fecf609f7,1,SuperSecret")
	ct, err := Encrypt(key, iv, plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct)%16 != 0 || len(ct) < len(plain) {
		t.Fatalf("ciphertext length %d", len(ct))
	}
	dec, err := Decrypt(key, iv, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(Unpad(dec), plain) {
		t.Fatalf("round trip failed: %q", dec)
	}
	got, err := RecoverIV(key, ct, Pad(plain))
	if err != nil || !bytes.Equal(got, iv) {
		t.Fatalf("recovered iv %x, want %x (%v)", got, iv, err)
	}
	if err := CheckCBC(key, plain, ct); err != nil {
		t.Fatal(err)
	}
}

func TestZeroPlaintextGivesIVDirectly(t *testing.T) {
	// With P1 = 0, C1 = E(IV): decrypting the first block yields the IV.
	key, iv := randBytes(t, 16), randBytes(t, 16)
	ct, _ := Encrypt(key, iv, make([]byte, 32))
	got, _ := RecoverIV(key, ct, make([]byte, 16))
	if !bytes.Equal(got, iv) {
		t.Fatalf("got %x want %x", got, iv)
	}
}

func TestCheckCBCRejectsWrongKey(t *testing.T) {
	key, iv := randBytes(t, 16), randBytes(t, 16)
	plain := bytes.Repeat([]byte{0xff}, 32)
	ct, _ := Encrypt(key, iv, plain)
	if err := CheckCBC(randBytes(t, 16), plain, ct); err == nil {
		t.Fatal("want error with wrong key")
	}
	if err := CheckCBC(key, plain, ct[:16]); err == nil {
		t.Fatal("want error with a single block")
	}
}

// Vectors captured from a real Cube (firmware 1.1.3) with the e: command.
// The key is not known to the test suite; what the vectors show is that the
// Cube's output is deterministic and chained: the first block of the
// 16-byte and 32-byte inputs is identical, the second block differs from
// the first although the plaintext blocks are equal. That is CBC with a
// fixed IV, not ECB and not a per-message IV.
func TestRealCubeVectorsShowFixedIVCBC(t *testing.T) {
	ff16 := "Cv1h/QXXctIky4XX0Nnd4g=="
	ff32 := "Cv1h/QXXctIky4XX0Nnd4vcOSdN1Lu5ieW0Au7RLQ+s="
	a, _ := base64.StdEncoding.DecodeString(ff16)
	b, _ := base64.StdEncoding.DecodeString(ff32)
	if len(a) != 16 || len(b) != 32 {
		t.Fatalf("lengths %d %d", len(a), len(b))
	}
	if !bytes.Equal(a, b[:16]) {
		t.Error("first blocks differ: IV is not fixed")
	}
	if bytes.Equal(b[:16], b[16:]) {
		t.Error("equal blocks: looks like ECB")
	}
}

// Two e: calls made by the original software on the same Cube: both
// plaintexts start with the same 16 bytes ("H:", the Cube's serial, ",0b9"), and so do
// the ciphertexts, which again shows a fixed IV.
func TestOriginalSoftwareVectorsShareFirstBlock(t *testing.T) {
	a, _ := base64.StdEncoding.DecodeString("MgMYlNDXbqm4PhERXHoyI63jvEzCs4JW9bJfuLRj3mc=")
	b, _ := base64.StdEncoding.DecodeString("MgMYlNDXbqm4PhERXHoyI7TkR6a30furU2UUMx9ekSrM0+6syRr8BxUnfXo2ghgiz5ST6wLMiCqKe+OpML32tetyIrXsaQ10+gVU/OCL9BBbpfUjZKdF38JsYr5/Sefi")
	if !bytes.Equal(a[:16], b[:16]) || bytes.Equal(a[16:32], b[16:32]) {
		t.Fatalf("unexpected block structure")
	}
}
