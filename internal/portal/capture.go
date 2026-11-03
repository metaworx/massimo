// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package portal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// DefaultReplier answers what the original portal was observed to answer:
// an i: line, plus b:OK when the request carried a B: (login) line.
// interval is the first field of the i: line; its meaning is unknown.
func DefaultReplier(iLine string, extra []cube.Message) Replier {
	return func(_ string, req []cube.Message) []cube.Message {
		out := []cube.Message{{Type: 'i', Data: iLine}}
		for _, m := range req {
			if m.Type == 'B' {
				out = append(out, cube.Message{Type: 'b', Data: "OK"})
			}
		}
		return append(out, extra...)
	}
}

// Capture is an HTTP handler that decrypts whatever a Cube sends, logs it
// and replies through a Replier. It is a research tool: it accepts any
// path and any serial for which it has a codec.
type Capture struct {
	Codecs map[string]*Codec // by serial; the key "" is used for unknown serials
	Reply  Replier
	Log    *slog.Logger
	Dir    string // if set, every exchange is appended to <Dir>/portal-capture.jsonl

	mu sync.Mutex
}

func (c *Capture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ex := &Exchange{Time: time.Now(), Remote: r.RemoteAddr, Method: r.Method, Path: r.URL.RequestURI(), Headers: map[string]string{}}
	for k, v := range r.Header {
		ex.Headers[k] = strings.Join(v, ", ")
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
	for _, m := range msgs {
		ex.Lines = append(ex.Lines, m.String())
	}
	if err != nil {
		ex.Error = "decode: " + err.Error()
		c.finish(w, ex, http.StatusInternalServerError, nil)
		return
	}
	reply := c.Reply(serial, msgs)
	for _, m := range reply {
		ex.ReplyLines = append(ex.ReplyLines, m.String())
	}
	enc, err := codec.Encode(reply)
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
		attrs := []any{"remote", ex.Remote, "path", ex.Path, "serial", ex.Headers["Max-Serial"], "status", ex.Status}
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
