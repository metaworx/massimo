// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

// dropsSYN says that a full accept queue makes the kernel drop a SYN
// without an answer, so that a refused client's dial times out as it
// does against a Cube that holds another session.
const dropsSYN = true

// fullBacklog is the backlog restored once the slot is free; the kernel
// caps it at net.core.somaxconn.
const fullBacklog = 4096

// setBacklog changes the backlog of a listening socket: Linux applies a
// second listen() to a socket that listens already.
func setBacklog(ln net.Listener, n int) error {
	tl, ok := ln.(*net.TCPListener)
	if !ok {
		return errors.ErrUnsupported
	}
	raw, err := tl.SyscallConn()
	if err != nil {
		return err
	}
	var lerr error
	if err := raw.Control(func(fd uintptr) { lerr = unix.Listen(int(fd), n) }); err != nil {
		return err
	}
	return lerr
}

// holdPort binds a socket that does not listen to the port of ln, a TCP
// listener, and returns it as the port's hold: ln can then close while
// the port stays bound, so that a dial is refused, as where nothing
// listens, and nothing else takes the port (Pause). The two share the
// port through SO_REUSEPORT, which holdPort sets on ln too; while they
// do, a socket can join them only where it sets it as well and belongs
// to the same user, and the kernel never gives the port to a socket
// that asks for any port. The listener of the hold's reopen sets none,
// so that outside a pause no socket can join at all. For the moment of
// the reopen, a listener with SO_REUSEADDR, as Go's are, can bind the
// port; Resume then fails and the fake stays paused, its port held. A
// program can aim at the port only where it learned its number before,
// as in the race of a port freed and taken again. Where holdPort fails,
// ln is as it was, and listens on.
func holdPort(ln net.Listener) (_ portHold, err error) {
	tl, ok := ln.(*net.TCPListener)
	if !ok {
		return nil, errors.ErrUnsupported
	}
	raw, err := tl.SyscallConn()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			raw.Control(func(fd uintptr) { unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 0) })
		}
	}()
	var sa unix.Sockaddr
	family, v6only := unix.AF_INET, 0
	var serr error
	err = raw.Control(func(fd uintptr) {
		if serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); serr != nil {
			return
		}
		if sa, serr = unix.Getsockname(int(fd)); serr != nil {
			return
		}
		if _, ok := sa.(*unix.SockaddrInet6); ok {
			family = unix.AF_INET6
			v6only, serr = unix.GetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_V6ONLY)
		}
	})
	if err = errors.Join(err, serr); err != nil {
		return nil, err
	}
	fd, err := unix.Socket(family, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	// SO_REUSEADDR for the bind alone: the connections ln accepted, and
	// their TIME_WAIT, share the port without SO_REUSEPORT. Cleared once
	// bound, it lets no listener of another program bind the port.
	err = errors.Join(unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1),
		unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1))
	if err == nil && family == unix.AF_INET6 {
		err = unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, v6only)
	}
	if err == nil {
		err = unix.Bind(fd, sa)
	}
	if err == nil {
		err = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 0)
	}
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	return &socketHold{fd: fd, addr: ln.Addr().String()}, nil
}

// socketHold is the hold of holdPort: a socket bound to the port, which
// does not listen, and the address of the listener it was bound beside.
type socketHold struct {
	fd   int
	addr string
}

// reopen listens on the held port again, with a listener as net.Listen
// opens it, without SO_REUSEPORT: SO_REUSEADDR on the holding socket
// again lets it bind beside it, and once the holding socket is released
// the port is the listener's alone, as before the pause. (A listener
// that set SO_REUSEPORT would keep the port open to others: the kernel
// keeps the port's sharing as long as a socket is bound to it, and
// clearing the option on a listener does not end it.)
func (h *socketHold) reopen() (net.Listener, error) {
	if err := unix.SetsockoptInt(h.fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", h.addr)
	if err != nil {
		unix.SetsockoptInt(h.fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 0)
		return nil, err
	}
	return ln, nil
}

// release closes the holding socket.
func (h *socketHold) release() { unix.Close(h.fd) }
