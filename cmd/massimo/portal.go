// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"go.metaworx.ch/massimo/internal/portal"
	"go.metaworx.ch/massimo/internal/portal/portalcrypto"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

const ivEnv = "MASSIMO_CUBE_IV"

// runPortal starts the capture server. It serves one Cube key (from the
// environment or a prompt) and the IV from -iv or MASSIMO_CUBE_IV, and
// replies like the original portal unless told otherwise.
func runPortal(args []string) error {
	fs := flag.NewFlagSet("portal", flag.ContinueOnError)
	var listens listenList
	fs.Var(&listens, "listen", "address to listen on, [ip]:port; repeat for several (default :8080)")
	ivHex := fs.String("iv", os.Getenv(ivEnv), "IV as 32 hex digits (or "+ivEnv+")")
	serial := fs.String("serial", "", "only accept this Cube serial (default: any serial, same key)")
	iLine := fs.String("i", "000051b1,00000000,ffffffff", "payload of the i: reply line; 'none' sends no i: line; {conn} is replaced by the Cube's connection id")
	extra := fs.String("extra", "", "extra reply lines, separated by ';'; {conn} and {now} are expanded (e.g. 'v:<tz>,{now}')")
	dir := fs.String("dir", ".", "directory for portal-capture.jsonl")
	deviceURL := fs.String("device-url", "", "URL returned by /lookup (default: http://<this host>:<port>/cube)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo portal [flags]\n\nFake MAX! portal: decrypts and logs what a Cube sends, replies like the original.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(listens) == 0 {
		listens = listenList{":8080"}
	}
	if *ivHex == "" {
		return fmt.Errorf("portal: need -iv or %s (recover it with 'portal-crypto iv')", ivEnv)
	}
	iv, err := portalcrypto.ParseIV(*ivHex)
	if err != nil {
		return err
	}
	key, err := readKey()
	if err != nil {
		return err
	}
	codec, err := portal.NewCodec(key, iv)
	if err != nil {
		return err
	}
	var extraMsgs []cube.Message
	for _, l := range strings.Split(*extra, ";") {
		if l = strings.TrimSpace(l); l != "" {
			m, err := cube.ParseLine(l)
			if err != nil {
				return fmt.Errorf("-extra: %w", err)
			}
			extraMsgs = append(extraMsgs, m)
		}
	}
	if *iLine == "none" {
		*iLine = ""
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cap := &portal.Capture{
		Codecs:     map[string]*portal.Codec{*serial: codec},
		Reply:      portal.DefaultReplier(*iLine, extraMsgs),
		Log:        logger,
		Dir:        *dir,
		DeviceURL:  *deviceURL,
		ListenPort: strings.TrimPrefix(portForURL(listens[0]), ":"),
	}
	cap.Version = versionString()
	// Listen on IPv4 explicitly: on Windows a dual-stack IPv6 listener can
	// bind next to another program's IPv4 socket on the same port and then
	// silently receive nothing, while IPv4 traffic goes to the other program.
	errc := make(chan error, len(listens))
	for _, addr := range listens {
		ln, err := net.Listen("tcp4", addr)
		if err != nil {
			return fmt.Errorf("portal: %w", err)
		}
		logger.Info("fake portal listening", "addr", ln.Addr().String(), "serial", orAny(*serial), "capture", *dir+"/portal-capture.jsonl")
		logger.Info("point the Cube here with: u:http://<this host>" + portForURL(addr) + "/cube")
		go func() { errc <- http.Serve(ln, cap) }()
	}
	return <-errc
}

// listenList collects repeated -listen flags.
type listenList []string

func (l *listenList) String() string     { return strings.Join(*l, ",") }
func (l *listenList) Set(v string) error { *l = append(*l, v); return nil }

func orAny(s string) string {
	if s == "" {
		return "(any)"
	}
	return s
}

func portForURL(listen string) string {
	if i := strings.LastIndex(listen, ":"); i >= 0 {
		return listen[i:]
	}
	return ""
}
