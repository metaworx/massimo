// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package portal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

var testKey = bytes.Repeat([]byte{0x11}, 16)
var testIV = bytes.Repeat([]byte{0x22}, 16)

func TestCodecRoundTrip(t *testing.T) {
	c, err := NewCodec(testKey, testIV)
	if err != nil {
		t.Fatal(err)
	}
	in := []cube.Message{{Type: 'H', Data: "KMD1016788,099c3e,0113"}, {Type: 'B', Data: "u,h,1,p"}}
	ct, err := c.Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct)%16 != 0 {
		t.Fatalf("ciphertext not block aligned: %d", len(ct))
	}
	out, text, err := c.Decode(ct)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[1].String() != "B:u,h,1,p" || text != "H:KMD1016788,099c3e,0113\r\nB:u,h,1,p\r\n" {
		t.Fatalf("got %v %q", out, text)
	}
}

func TestCaptureLoginExchange(t *testing.T) {
	dir := t.TempDir()
	codec, _ := NewCodec(testKey, testIV)
	cap := &Capture{
		Codecs: map[string]*Codec{"KMD1016788": codec},
		Reply:  DefaultReplier("000051b1,00000000,ffffffff", nil),
		Dir:    dir,
	}
	srv := httptest.NewServer(cap)
	defer srv.Close()

	body, _ := codec.Encode([]cube.Message{
		{Type: 'H', Data: "KMD1016788,099c3e,0113"},
		{Type: 'B', Data: "FOOBAR1,5a8a4a5d3c1bd612b8bf1e2fecf609f7,1,SuperSecret"},
	})
	req, _ := http.NewRequest("POST", srv.URL+"/cube", bytes.NewReader(body))
	req.Header.Set("MAX-Serial", "KMD1016788")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var rb bytes.Buffer
	rb.ReadFrom(resp.Body)
	msgs, _, err := codec.Decode(rb.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].String() != "i:000051b1,00000000,ffffffff" || msgs[1].String() != "b:OK" {
		t.Fatalf("reply %v", msgs)
	}

	// Unknown serial without a fallback codec is rejected and still logged.
	req, _ = http.NewRequest("POST", srv.URL+"/cube", bytes.NewReader(body))
	req.Header.Set("MAX-Serial", "XXX0000000")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("unknown serial: status %d", resp.StatusCode)
	}

	f, err := os.ReadFile(filepath.Join(dir, "portal-capture.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(f), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("want 2 capture records, got %d", len(lines))
	}
	var ex Exchange
	if err := json.Unmarshal(lines[0], &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Lines[1][:2] != "B:" || ex.ReplyLines[1] != "b:OK" || ex.Headers["Max-Serial"] != "KMD1016788" {
		t.Fatalf("record: %+v", ex)
	}
}

func TestReplierConnPlaceholder(t *testing.T) {
	r := DefaultReplier("{conn},00000000,ffffffff", nil)
	req := []cube.Message{{Type: 'H', Data: "KEQ0537249,0b9573,0113,00000000,4470fa70,44,32,1a0b03,1602,03,0000"}}
	got := r("KEQ0537249", req)
	if len(got) != 1 || got[0].String() != "i:4470fa70,00000000,ffffffff" {
		t.Fatalf("got %v", got)
	}
	if got := DefaultReplier("", nil)("x", req); len(got) != 0 {
		t.Fatalf("empty iLine must omit i:, got %v", got)
	}
}

func TestExpandNowPlaceholder(t *testing.T) {
	got := expandPlaceholders("v:abc,{now}", nil)
	if len(got) != len("v:abc,")+8 || got[:6] != "v:abc," {
		t.Fatalf("got %q", got)
	}
	if _, err := strconv.ParseUint(got[6:], 16, 32); err != nil {
		t.Fatalf("not hex: %q", got)
	}
}
