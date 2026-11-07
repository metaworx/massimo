// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package portal

import (
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

func hello(slots int) []cube.Message {
	return []cube.Message{{Type: 'H', Data: "KEQ0000100,0a0107,0113,00000000,53c00ada,00," + hexByte(slots) + ",1a0b04,091e,03,0000"}}
}

func hexByte(n int) string {
	return string("0123456789abcdef"[n>>4]) + string("0123456789abcdef"[n&15])
}

func TestProberJudgesBySlots(t *testing.T) {
	p := &Prober{
		Variants: []Variant{{"a", "{i}\r\ns:{s}\r\n"}, {"b", "s:{s}\n"}},
		Values:   map[string]string{"s": "AAA="},
		Default:  "i:x\r\n",
		ILine:    "000051b1,00000000,ffffffff",
	}
	text, lines := p.Reply("KEQ0000100", hello(50))
	if text != "i:000051b1,00000000,ffffffff\r\ns:AAA=\r\n" || len(lines) != 2 || lines[1] != `s:AAA=\r\n` {
		t.Fatalf("first reply %q %v", text, lines)
	}
	// Second poll: slots dropped → variant a accepted; variant b goes out.
	text, _ = p.Reply("KEQ0000100", hello(49))
	if text != "s:AAA=\n" {
		t.Fatalf("second reply %q", text)
	}
	// Third poll: slots unchanged → b had no effect; default from now on.
	text, _ = p.Reply("KEQ0000100", hello(49))
	if text != "i:x\r\n" || !p.Done() {
		t.Fatalf("third reply %q done=%v", text, p.Done())
	}
	res := p.Results()
	if len(res) != 2 || !res[0].Accepted || res[0].SlotsFrom != 50 || res[0].SlotsTo != 49 || res[1].Accepted {
		t.Fatalf("results %+v", res)
	}
	vs, err := ParseVariants("# comment\nx = {i}\\r\\ns:{s}\\r\\n\n")
	if err != nil || len(vs) != 1 || vs[0].Text != "{i}\r\ns:{s}\r\n" {
		t.Fatalf("parse %+v %v", vs, err)
	}
	if n := len(DefaultVariants()); n < 10 {
		t.Fatalf("default variants %d", n)
	}
}
