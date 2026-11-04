// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Message is one protocol line: a single type character, a colon and a
// payload, terminated by CR LF on the wire. Lowercase types are commands sent
// to the Cube, uppercase types are messages sent by the Cube.
type Message struct {
	Type byte
	Data string
}

// String returns the line without the terminating CR LF.
func (m Message) String() string { return string(m.Type) + ":" + m.Data }

// Wire returns the line as sent on the wire, including CR LF.
func (m Message) Wire() string { return m.String() + "\r\n" }

// ErrNotAMessage is returned for lines that do not have the "X:" form.
var ErrNotAMessage = errors.New("not a cube protocol line")

// ParseLine parses a single line. Trailing CR and LF are ignored.
func ParseLine(line string) (Message, error) {
	line = strings.TrimRight(line, "\r\n")
	if len(line) < 2 || line[1] != ':' {
		return Message{}, fmt.Errorf("%w: %q", ErrNotAMessage, truncate(line, 40))
	}
	return Message{Type: line[0], Data: line[2:]}, nil
}

// SplitLines splits a block of text into protocol messages. Both CR LF and LF
// separators are accepted, and trailing NUL padding (as left by AES zero
// padding) is removed. Empty lines are skipped.
func SplitLines(text string) ([]Message, error) {
	text = strings.TrimRight(text, "\x00")
	var out []Message
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, "\r")
		if l == "" {
			continue
		}
		m, err := ParseLine(l)
		if err != nil {
			return out, err
		}
		out = append(out, m)
	}
	return out, nil
}

// Reader reads protocol lines from a stream.
type Reader struct {
	r *bufio.Reader
}

// NewReader returns a Reader. Lines may be several kilobytes long (M
// messages), so the buffer is sized generously.
func NewReader(r io.Reader) *Reader { return &Reader{r: bufio.NewReaderSize(r, 16<<10)} }

// ReadRaw returns the next line without CR LF.
func (r *Reader) ReadRaw() (string, error) {
	line, err := r.r.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return strings.TrimRight(line, "\r\n"), nil
		}
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// Read returns the next message.
func (r *Reader) Read() (Message, error) {
	line, err := r.ReadRaw()
	if err != nil {
		return Message{}, err
	}
	return ParseLine(line)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// b64 is the encoding used throughout the protocol (standard alphabet, padded).
var b64 = base64.StdEncoding

func decodeB64(field, s string) ([]byte, error) {
	b, err := b64.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("%s: invalid base64: %w", field, err)
	}
	return b, nil
}

// Until is the end of a temporary ("vacation") setting. MAX! devices store it
// as a date plus a time in 30-minute steps; there is no time zone.
type Until struct {
	Year, Month, Day int
	Minutes          int // minutes since midnight, multiple of 30
}

// IsZero reports whether u is unset.
func (u Until) IsZero() bool { return u == Until{} }

// Time returns u as a time in the given location.
func (u Until) Time(loc *time.Location) time.Time {
	return time.Date(u.Year, time.Month(u.Month), u.Day, u.Minutes/60, u.Minutes%60, 0, 0, loc)
}

// UntilFromTime rounds t down to the previous half hour.
func UntilFromTime(t time.Time) Until {
	return Until{Year: t.Year(), Month: int(t.Month()), Day: t.Day(), Minutes: t.Hour()*60 + t.Minute()/30*30}
}

func (u Until) String() string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d", u.Year, u.Month, u.Day, u.Minutes/60, u.Minutes%60)
}

// MarshalText implements encoding.TextMarshaler.
func (u Until) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

// UnmarshalText parses the form written by String ("2026-11-04 13:30").
func (u *Until) UnmarshalText(b []byte) error {
	var y, mo, d, h, mi int
	if _, err := fmt.Sscanf(string(b), "%d-%d-%d %d:%d", &y, &mo, &d, &h, &mi); err != nil {
		return fmt.Errorf("until %q: want YYYY-MM-DD HH:MM", string(b))
	}
	*u = Until{Year: y, Month: mo, Day: d, Minutes: h*60 + mi}
	return nil
}

// decodeUntilDate decodes the 2-byte date. The bit layout follows FHEM's
// implementation, which matches the example in the protocol docs:
//
//	byte 0: MMMD DDDD  (month bits 3..1, day)
//	byte 1: MxYY YYYY  (month bit 0, year - 2000)
func decodeUntilDate(b0, b1 byte) (year, month, day int) {
	day = int(b0 & 0x1f)
	month = int(b0&0xe0)>>4 | int(b1>>7)
	year = 2000 + int(b1&0x3f)
	return
}

func encodeUntilDate(year, month, day int) (byte, byte, error) {
	if year < 2000 || year > 2063 || month < 1 || month > 12 || day < 1 || day > 31 {
		return 0, 0, fmt.Errorf("until date %04d-%02d-%02d out of range", year, month, day)
	}
	b0 := byte((month&0x0e)<<4) | byte(day)
	b1 := byte((month&0x01)<<7) | byte(year-2000)
	return b0, b1, nil
}

func encodeUntil(u Until) ([]byte, error) {
	b0, b1, err := encodeUntilDate(u.Year, u.Month, u.Day)
	if err != nil {
		return nil, err
	}
	if u.Minutes < 0 || u.Minutes >= 24*60 || u.Minutes%30 != 0 {
		return nil, fmt.Errorf("until time %d min: must be a multiple of 30 minutes within a day", u.Minutes)
	}
	return []byte{b0, b1, byte(u.Minutes / 30)}, nil
}
