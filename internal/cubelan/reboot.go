// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"strings"
	"time"
)

// RebootWait is how long Reboot waits for the Cube's answer to R.
const RebootWait = time.Second

// RebootWindow is how long a client waits for its Cube's restart
// (Rebooting): a Cube answered ping again within 43 s of an R and took
// a connection 61 s after it (2026-11-13, firmware 1.1.3).
const RebootWindow = 3 * time.Minute

// RebootResult is what Reboot learned of the R it sent.
type RebootResult struct {
	// Answered says that the Cube answered within RebootWait; on the box
	// of a serve that holds the discovery port its answer goes there.
	Answered bool
	// Status is the answer's status: 2, restarting, on firmware 1.1.3.
	Status byte
}

// Reboot sends R, the request that restarts the Cube with the given
// serial (RebootFrame), from pc to addr (host or host:port, the port
// 23272 by default), and waits up to RebootWait for the answer. packets
// are pc's datagrams where another reader owns the socket, as serve's
// responder does; nil has Reboot read it. A restart drops the
// radiograms the Cube has not delivered (docs/cube-protocol.md), so it
// is sent on a person's word only.
func Reboot(ctx context.Context, pc net.PacketConn, packets <-chan Packet, addr, serial string) (RebootResult, error) {
	to, err := udpTarget(addr)
	if err != nil {
		return RebootResult{}, err
	}
	if packets == nil {
		ch := make(chan Packet, 16)
		done := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			readPackets(pc, ch, done)
		}()
		defer func() {
			close(done)
			<-finished
		}()
		packets = ch
	}
	if _, err := pc.WriteTo(RebootFrame(serial), to); err != nil {
		return RebootResult{}, err
	}
	timer := time.NewTimer(RebootWait)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return RebootResult{}, ctx.Err()
		case <-timer.C:
			return RebootResult{}, nil
		case p := <-packets:
			s, _, op, data, ok := parseUDPReply(p.Data)
			if !ok || op != udpReboot || !strings.EqualFold(strings.Trim(s, "* "), strings.TrimSpace(serial)) {
				continue
			}
			res := RebootResult{Answered: true}
			if len(data) > 0 {
				res.Status = data[0]
			}
			return res, nil
		}
	}
}

// Rebooting tells the client that its Cube was told to restart: a
// connection, live or held, is closed as Reconnect closes one, without a
// loss warning, and the link is rebooting until the next greeting, for d
// at most, while the client dials with its backoff.
func (c *Client) Rebooting(d time.Duration) {
	now := time.Now()
	c.heardMu.Lock()
	c.rebootAt, c.rebootUntil = now, now.Add(d)
	c.heardMu.Unlock()
	c.log.Info("cube rebooting", "addr", c.Address()) // no event says it
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		c.reconnecting.Store(true)
		conn.Close()
	}
}

// rebooting reports whether a reboot is being waited for.
func (c *Client) rebooting(now time.Time) bool {
	c.heardMu.Lock()
	defer c.heardMu.Unlock()
	return now.Before(c.rebootUntil)
}

// endReboot ends the wait for a reboot: the Cube greeted.
func (c *Client) endReboot() {
	c.heardMu.Lock()
	c.rebootAt, c.rebootUntil = time.Time{}, time.Time{}
	c.heardMu.Unlock()
}

// SendReboot is Reboot from a socket of its own: on the discovery port
// where it is free, since the Cube answers there only, else on any port,
// the R going out all the same and its answer elsewhere (to the serve
// that holds the port).
func SendReboot(ctx context.Context, addr, serial string) (RebootResult, error) {
	pc, err := listenDiscovery()
	if err != nil {
		return RebootResult{}, err
	}
	defer pc.Close()
	return Reboot(ctx, pc, nil, addr, serial)
}
