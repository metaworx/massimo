// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package portal

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Variant is one reply to try on the Cube. Text is the complete reply
// plaintext; the placeholders {i}, {serial}, {rf}, {fw} and any extra
// ones given to the Prober are expanded.
type Variant struct {
	Name string
	Text string
}

// ProbeResult is the verdict for one variant, taken from the H line of
// the poll that followed it.
type ProbeResult struct {
	Variant    string
	SentAt     time.Time
	SlotsFrom  int
	SlotsTo    int
	DutyFrom   int
	DutyTo     int
	Accepted   bool // the free memory slots went down: the Cube queued a radiogram
	NextPollAt time.Time
}

// Prober replies to each poll with the next variant and judges the
// previous one by the Cube's budget. The free memory slots are the
// indicator: every radio command the Cube accepts occupies one slot until
// a device acknowledges it, while the Cube's own retries only move the
// duty cycle. After the last variant the prober replies with Default.
type Prober struct {
	Variants []Variant
	Values   map[string]string // extra placeholders, e.g. "s" → base64 payload
	Default  string            // reply text once every variant was sent
	ILine    string            // expands {i}
	Log      *slog.Logger
	// Trigger, if set, is called Delay after each probe reply so the
	// next poll comes soon (e.g. by sending i: over the LAN).
	Trigger func()
	Delay   time.Duration

	mu      sync.Mutex
	next    int
	pending *ProbeResult // the variant waiting for its verdict
	results []ProbeResult
}

// Results returns the verdicts so far.
func (p *Prober) Results() []ProbeResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ProbeResult(nil), p.results...)
}

// Done reports whether every variant was sent and judged.
func (p *Prober) Done() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.next >= len(p.Variants) && p.pending == nil
}

// Reply records the verdict for the previous variant and returns the
// next reply text together with its lines for the log.
func (p *Prober) Reply(serial string, req []cube.Message) (text string, lines []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var h *cube.Hello
	for _, m := range req {
		if m.Type == 'H' {
			if hh, err := cube.ParseHello(m.Data); err == nil {
				h = hh
			}
		}
	}
	if h == nil {
		return p.Default, []string{"(no H line in this request; nothing probed)"}
	}
	if p.pending != nil {
		r := *p.pending
		r.SlotsTo, r.DutyTo, r.NextPollAt = h.FreeMemorySlots, h.DutyCycle, time.Now()
		r.Accepted = r.SlotsTo < r.SlotsFrom
		p.results = append(p.results, r)
		p.pending = nil
		if p.Log != nil {
			verdict := "no effect"
			if r.Accepted {
				verdict = "ACCEPTED: a radiogram was queued"
			}
			p.Log.Info("probe verdict", "variant", r.Variant, "slots", fmt.Sprintf("%d→%d", r.SlotsFrom, r.SlotsTo),
				"duty_pct", fmt.Sprintf("%d→%d", r.DutyFrom, r.DutyTo), "after", r.NextPollAt.Sub(r.SentAt).Round(time.Second), "result", verdict)
		}
	}
	if p.next >= len(p.Variants) {
		if p.Log != nil && p.next == len(p.Variants) {
			p.next++ // log the summary once
			p.Log.Info("probe finished", "variants", len(p.Variants), "accepted", p.acceptedNamesLocked())
		}
		return p.Default, splitLines(p.Default)
	}
	v := p.Variants[p.next]
	p.next++
	text = p.expand(v.Text, h)
	p.pending = &ProbeResult{Variant: v.Name, SentAt: time.Now(), SlotsFrom: h.FreeMemorySlots, DutyFrom: h.DutyCycle}
	if p.Log != nil {
		p.Log.Info("probe", "variant", fmt.Sprintf("%d/%d %s", p.next, len(p.Variants), v.Name), "slots", h.FreeMemorySlots, "duty_pct", h.DutyCycle)
	}
	if p.Trigger != nil {
		go func() {
			time.Sleep(p.Delay)
			p.Trigger()
		}()
	}
	return text, splitLines(text)
}

func (p *Prober) acceptedNamesLocked() string {
	var names []string
	for _, r := range p.results {
		if r.Accepted {
			names = append(names, r.Variant)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ",")
}

func (p *Prober) expand(text string, h *cube.Hello) string {
	r := strings.NewReplacer("{i}", "i:"+p.ILine, "{serial}", h.Serial, "{rf}", h.RFAddress.String(), "{fw}", h.Firmware, "{conn}", h.ConnectionID)
	text = r.Replace(text)
	for k, v := range p.Values {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return strings.ReplaceAll(text, "\\n", "\n")
}

// splitLines renders reply text for the log, making line endings visible.
func splitLines(text string) []string {
	var out []string
	for _, l := range strings.SplitAfter(text, "\n") {
		if l == "" {
			continue
		}
		l = strings.ReplaceAll(strings.ReplaceAll(l, "\r", "\\r"), "\n", "\\n")
		out = append(out, l)
	}
	return out
}

// ParseVariants reads variants from text: one per line as "name=text",
// where text uses \r and \n escapes. Blank lines and # comments are skipped.
func ParseVariants(src string) ([]Variant, error) {
	var out []Variant
	for n, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, text, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected name=text", n+1)
		}
		out = append(out, Variant{Name: strings.TrimSpace(name), Text: unescape(strings.TrimSpace(text))})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no variants")
	}
	return out, nil
}

func unescape(s string) string {
	return strings.NewReplacer(`\r`, "\r", `\n`, "\n").Replace(s)
}

// DefaultVariants are the reply forms worth trying for a radio command
// {s} (base64 payload of an s: command), {shex} (the same as hex) and
// {r} (the data of an r: command). Controls at both ends show the
// baseline. CRLF everywhere unless the name says otherwise.
func DefaultVariants() []Variant {
	return []Variant{
		{"control-i-only", "{i}\r\n"},
		{"i-then-s", "{i}\r\ns:{s}\r\n"},
		{"s-only", "s:{s}\r\n"},
		{"s-then-i", "s:{s}\r\n{i}\r\n"},
		{"with-H-header", "H:{serial},{rf},{fw}\r\n{i}\r\ns:{s}\r\n"},
		{"lf-only", "{i}\ns:{s}\n"},
		{"no-final-crlf", "{i}\r\ns:{s}"},
		{"uppercase-S", "{i}\r\nS:{s}\r\n"},
		{"hex-payload", "{i}\r\ns:{shex}\r\n"},
		{"r-reset-error", "{i}\r\nr:{r}\r\n"},
		{"a-then-s", "a:\r\ns:{s}\r\n"},
		{"control-i-only-end", "{i}\r\n"},
	}
}
