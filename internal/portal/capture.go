// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package portal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Exchange is one request/response pair, as written to the capture log.
type Exchange struct {
	Time       time.Time         `json:"time"`
	Remote     string            `json:"remote"`
	Local      string            `json:"local"` // the listener address the request arrived on
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	BodyB64    string            `json:"body_b64"`
	Plaintext  string            `json:"plaintext,omitempty"`
	Lines      []string          `json:"lines,omitempty"`
	Error      string            `json:"error,omitempty"`
	ReplyLines []string          `json:"reply_lines,omitempty"`
	ReplyB64   string            `json:"reply_b64,omitempty"`
	Status     int               `json:"status"`
}

// Replier decides what to answer to a decoded request.
type Replier func(serial string, req []cube.Message) []cube.Message

// expandPlaceholders replaces {conn} with the connection id from the Cube's
// H line and {now} with the current time as the v: command expects it
// (seconds since 2000-01-01 UTC, 8 hex digits).
func expandPlaceholders(line string, req []cube.Message) string {
	if strings.Contains(line, "{conn}") {
		conn := "00000000"
		for _, m := range req {
			if m.Type == 'H' {
				if h, err := cube.ParseHello(m.Data); err == nil && h.ConnectionID != "" {
					conn = h.ConnectionID
				}
			}
		}
		line = strings.ReplaceAll(line, "{conn}", conn)
	}
	if strings.Contains(line, "{now}") {
		line = strings.ReplaceAll(line, "{now}", cube.CubeTime(time.Now()))
	}
	return line
}

// account is what the original portal knew about a Cube's remote access.
type account struct {
	user    string
	enabled bool
}

// DefaultReplier answers what the original portal was observed to answer:
// an i: line, plus the replies the original PC software expects for its
// account requests (B: login → b:OK, J: → j:OK, G: → g:<user>,<enabled>).
// An empty iLine omits the i: line; its fields' meaning is unknown. The
// placeholder {conn} is replaced by the connection id from the Cube's H line.
func DefaultReplier(iLine string, extra []cube.Message) Replier {
	var mu sync.Mutex
	accounts := map[string]*account{}
	return func(serial string, req []cube.Message) []cube.Message {
		mu.Lock()
		defer mu.Unlock()
		acc := accounts[serial]
		if acc == nil {
			acc = &account{}
			accounts[serial] = acc
		}
		var out []cube.Message
		if iLine != "" {
			out = append(out, cube.Message{Type: 'i', Data: expandPlaceholders(iLine, req)})
		}
		for _, m := range req {
			switch m.Type {
			case 'B': // B:<user>,<md5>,<flag>,<password>
				if user, _, ok := strings.Cut(m.Data, ","); ok {
					acc.user = user
				}
				out = append(out, cube.Message{Type: 'b', Data: "OK"})
			case 'J':
				acc.enabled = m.Data == "1"
				out = append(out, cube.Message{Type: 'j', Data: "OK"})
			case 'G':
				out = append(out, cube.Message{Type: 'g', Data: fmt.Sprintf("%s,%t", acc.user, acc.enabled)})
			}
		}
		for _, m := range extra {
			out = append(out, cube.Message{Type: m.Type, Data: expandPlaceholders(m.Data, req)})
		}
		return out
	}
}

// Capture is an HTTP handler that decrypts whatever a Cube sends, logs it
// and replies through a Replier. It is a research tool: it accepts any
// path and any serial for which it has a codec.
type Capture struct {
	Codecs map[string]*Codec // by serial; the key "" is used for unknown serials
	Reply  Replier
	// Probe, if set, replaces Reply: each poll gets the next variant.
	Probe *Prober
	Log   *slog.Logger
	Dir   string // if set, every exchange is appended to <Dir>/portal-capture.jsonl

	// DeviceURL is what /lookup returns; empty derives http://<host>:<ListenPort>/cube
	// from the request's Host header.
	DeviceURL  string
	ListenPort string // e.g. "8080"; used when DeviceURL is empty
	Version    string // shown on the status page

	mu       sync.Mutex
	started  time.Time
	requests int
	lastPoll map[string]time.Time
}

// statusPage answers browsers and other GETs without a body with a short
// text page, so that opening the portal address in a browser shows what is
// running instead of offering a download.
func (c *Capture) statusPage(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	if c.started.IsZero() {
		c.started = time.Now()
	}
	started, requests := c.started, c.requests
	polls := make([]string, 0, len(c.lastPoll))
	for serial, t := range c.lastPoll {
		polls = append(polls, fmt.Sprintf("  %s  last poll %s (%s ago)", serial, t.Format(time.RFC3339), time.Since(t).Round(time.Second)))
	}
	c.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "%s\nfake MAX! portal, up since %s, %d portal requests\n", c.Version, started.Format(time.RFC3339), requests)
	fmt.Fprintf(w, "endpoints: POST /cube (Cube polls), GET /lookup (device URL)\n")
	if len(polls) > 0 {
		fmt.Fprintln(w, "cubes:")
		for _, p := range polls {
			fmt.Fprintln(w, p)
		}
	}
}

func (c *Capture) devicePort() string {
	if c.ListenPort == "" || c.ListenPort == "80" {
		return ""
	}
	return ":" + c.ListenPort
}

func (c *Capture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ex := &Exchange{Time: time.Now(), Remote: r.RemoteAddr, Local: localAddr(r), Method: r.Method, Path: r.URL.RequestURI(), Headers: map[string]string{}}
	for k, v := range r.Header {
		ex.Headers[k] = strings.Join(v, ", ")
	}
	if r.Method == http.MethodGet && !strings.HasSuffix(r.URL.Path, "/lookup") {
		c.statusPage(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/lookup") {
		// The original PC software asked www.max-portal.eq-3.de/lookup for
		// the Cube's portal URL before any portal request. With a hosts
		// entry pointing that name here, it gets ours.
		url := c.DeviceURL
		if url == "" {
			host, _, _ := net.SplitHostPort(r.Host)
			if host == "" {
				host = r.Host
			}
			url = "http://" + host + c.devicePort() + "/cube"
		}
		ex.Plaintext = url
		w.Header().Set("Content-Location", url)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, url)
		ex.Status = http.StatusOK
		c.record(ex)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		ex.Error = "read body: " + err.Error()
		c.finish(w, ex, http.StatusBadRequest, nil)
		return
	}
	ex.BodyB64 = base64.StdEncoding.EncodeToString(body)
	serial := r.Header.Get("MAX-Serial")

	codec := c.Codecs[serial]
	if codec == nil {
		codec = c.Codecs[""]
	}
	if codec == nil {
		ex.Error = fmt.Sprintf("no key for serial %q", serial)
		c.finish(w, ex, http.StatusInternalServerError, nil)
		return
	}
	if len(body) == 0 {
		ex.Error = "empty body"
		c.finish(w, ex, http.StatusOK, nil)
		return
	}
	msgs, text, err := codec.Decode(body)
	ex.Plaintext = text
	c.mu.Lock()
	if c.started.IsZero() {
		c.started = time.Now()
	}
	c.requests++
	if c.lastPoll == nil {
		c.lastPoll = map[string]time.Time{}
	}
	c.lastPoll[serial] = time.Now()
	c.mu.Unlock()
	for _, m := range msgs {
		ex.Lines = append(ex.Lines, m.String())
	}
	if err != nil {
		ex.Error = "decode: " + err.Error()
		c.finish(w, ex, http.StatusInternalServerError, nil)
		return
	}
	var enc []byte
	if c.Probe != nil {
		text, lines := c.Probe.Reply(serial, msgs)
		ex.ReplyLines = lines
		enc, err = codec.EncodeText(text)
	} else {
		reply := c.Reply(serial, msgs)
		for _, m := range reply {
			ex.ReplyLines = append(ex.ReplyLines, m.String())
		}
		enc, err = codec.Encode(reply)
	}
	if err != nil {
		ex.Error = "encode reply: " + err.Error()
		c.finish(w, ex, http.StatusInternalServerError, nil)
		return
	}
	ex.ReplyB64 = base64.StdEncoding.EncodeToString(enc)
	c.finish(w, ex, http.StatusOK, enc)
}

func (c *Capture) finish(w http.ResponseWriter, ex *Exchange, status int, body []byte) {
	ex.Status = status
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Connection", "close")
	w.WriteHeader(status)
	if body != nil {
		w.Write(body)
	}
	c.record(ex)
}

func (c *Capture) record(ex *Exchange) {
	if c.Log != nil {
		attrs := []any{"remote", ex.Remote, "local", ex.Local, "path", ex.Path, "serial", ex.Headers["Max-Serial"], "status", ex.Status}
		if ex.Error != "" {
			attrs = append(attrs, "error", ex.Error)
		}
		c.Log.Info("request", attrs...)
		for _, l := range ex.Lines {
			c.Log.Info("  cube  → " + l)
		}
		for _, l := range ex.ReplyLines {
			c.Log.Info("  reply ← " + l)
		}
		if ex.Error != "" && ex.Plaintext == "" && ex.BodyB64 != "" {
			c.Log.Info("  raw body (base64): " + ex.BodyB64)
		}
	}
	if c.Dir == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(c.Dir, "portal-capture.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		if c.Log != nil {
			c.Log.Error("capture file", "error", err)
		}
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(ex)
}

// localAddr returns the address of the listener that accepted the request.
func localAddr(r *http.Request) string {
	if a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		return a.String()
	}
	return ""
}
