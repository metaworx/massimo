// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go.metaworx.ch/massimo/internal/announce"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/serve"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/update"
)

// runServe starts the HTTP API over every configured Cube.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listenDefault := os.Getenv("MASSIMO_LISTEN")
	if listenDefault == "" {
		listenDefault = "127.0.0.1:8099"
	}
	listen := fs.String("listen", listenDefault, "address to listen on (default MASSIMO_LISTEN, else 127.0.0.1:8099)")
	addr := fs.String("addr", "", "serve this one Cube (host[:port]) instead of the configured ones, for this run and without the store: massimo.db is neither opened nor created, and nothing is written")
	poll := fs.Duration("poll", server.DefaultPollInterval, "how often to poll the device list (default the store's poll_interval, else 1m)")
	refresh := fs.Duration("refresh", server.DefaultRefreshInterval, "how often to reconnect to each Cube to re-read everything it holds (0 = never; default the store's refresh_interval, else 6h)")
	discoverEvery := fs.Duration("discover-interval", server.DefaultDiscoverInterval, "how often to look for Cubes on the LAN, with a first run at the start (0 = only on request through the API; default the store's discover_interval, else 30m)")
	var discoverTargets []string
	fs.Func("discover-target", "send discovery requests to this host[:port] instead of broadcasting (repeatable; for another subnet, or a fake Cube's -discovery)", func(v string) error {
		if _, _, err := net.SplitHostPort(v); err != nil {
			v = net.JoinHostPort(v, "23272")
		}
		discoverTargets = append(discoverTargets, v)
		return nil
	})
	discoverListen := fs.String("discover-listen", "", "the local address of the discovery socket, for the searches and the proxy's answers, instead of :23272, the only port the Cubes answer to and the original software asks; for tests, with -discover-target (a fake Cube's -discovery answers to any port)")
	token := fs.String("token", os.Getenv("MASSIMO_API_TOKEN"), "API token required as Bearer token (default MASSIMO_API_TOKEN; empty = no authentication)")
	logPath := fs.String("log", os.Getenv("MASSIMO_LOG"), "append the log (connections, commands, device state changes) to this file as well as stderr (default MASSIMO_LOG)")
	logLevelFlag := fs.String("log-level", os.Getenv(env.LogLevel), "log at this level and above: "+logLevelNames+" (default "+env.LogLevel+", else info)")
	var proxies []string
	fs.Func("proxy", "answer as a Cube on this address, for this run (repeatable; default the store's proxy address per Cube, else :62910 with a single Cube; \"off\" disables; with several Cubes give <serial>=<addr>)", func(v string) error {
		proxies = append(proxies, v)
		return nil
	})
	var showAll bool
	fs.BoolVar(&showAll, "show-hidden", false, "show the hidden devices (those of room 255 and of the Cube's hidden list) in the API and to proxy clients")
	fs.BoolVar(&showAll, "show-ghosts", false, "older name of -show-hidden")
	proxySerial := fs.String("proxy-serial", "", "serial to use in UDP discovery replies instead of the Cube's, for this run (default the store's proxy serial; the original software keeps one answer per serial)")
	updateRequest := fs.String("update-request", os.Getenv("MASSIMO_UPDATE_REQUEST"), "with the update helper installed: the request file that asks it for an update, such as "+update.DefaultRequestPath+"; on Windows the server then starts the helper itself (default MASSIMO_UPDATE_REQUEST; empty = updates cannot be started from the API)")
	updateStatus := fs.String("update-status", defaultUpdateStatus(), "the update helper's status file (on Windows install passes it, and the helper's update.log goes next to it)")
	tlsFlag := fs.String("tls", os.Getenv(tlsVar), "serve over TLS: off; files for the certificate of -tls-cert and -tls-key; self-signed for one the server makes for the box's names in the data directory's tls/; acme for one from Let's Encrypt for the names of -tls-names (default "+tlsVar+", else off; files when -tls-cert is given)")
	tlsCert := fs.String("tls-cert", os.Getenv(tlsCertVar), "the certificate for -tls files, PEM, the chain after the server's certificate; re-read when it changes (default "+tlsCertVar+")")
	tlsKey := fs.String("tls-key", os.Getenv(tlsKeyVar), "the certificate's private key, PEM (default "+tlsKeyVar+")")
	tlsNames := fs.String("tls-names", os.Getenv(tlsNamesVar), "with -tls self-signed, names and addresses the certificate holds besides the box's own; with -tls acme, the DNS names to get a certificate for; comma-separated (default "+tlsNamesVar+")")
	acmeEmail := fs.String("acme-email", os.Getenv(acmeEmailVar), "with -tls acme, the account's contact address, which the CA writes to about its certificates (default "+acmeEmailVar+")")
	acmeDirectory := fs.String("acme-directory", os.Getenv(acmeDirectoryVar), "with -tls acme, the ACME directory URL (default "+acmeDirectoryVar+", else Let's Encrypt's; its staging directory for experiments)")
	tlsHTTP := fs.String("tls-http", os.Getenv(tlsHTTPVar), "with -tls acme, also listen on this plain address, such as :80, for the HTTP-01 challenge and a redirect to https:// (default "+tlsHTTPVar+"; without it the TLS-ALPN-01 challenge on the TLS listener alone)")
	trustedProxies := fs.String("trusted-proxies", os.Getenv(trustedProxiesVar), "the reverse proxies whose X-Forwarded-For and X-Real-IP name the client, addresses and CIDRs separated by commas, such as 127.0.0.1 for a proxy on this box (default "+trustedProxiesVar+"; empty: the headers are ignored, anyone can set them)")
	acmeDNSHook := fs.String("acme-dns-hook", os.Getenv(acmeDNSHookVar), "with -tls acme, get the certificate by the DNS-01 challenge through this executable, run as '<hook> set <record> <txt>' and '<hook> clear <record> <txt>' (packaging/acme-dns-hooks/); no port needed, wildcards allowed (default "+acmeDNSHookVar+")")
	sighupMode := fs.String("sighup", os.Getenv(sighupVar), "what SIGHUP does on Unix: reload reads the settings file again, applying MASSIMO_API_TOKEN at once, and checkpoints the store; stop stops as SIGTERM does; auto reloads under a service manager and where SIGHUP was ignored at the start (nohup), and stops in a terminal, where SIGHUP means it closed (default "+sighupVar+", else auto)")
	restartMode := fs.String("restart", os.Getenv(restartVar), "what a restart from the API and the web UI does: auto offers it under a service manager that starts the server again (systemd, launchd, Homebrew, the Windows service); exit stops and exits with status 75 for whatever runs serve, such as Docker's --restart; off never offers it (default "+restartVar+", else auto)")
	allowReboot, rebootErr := newSwitchFlag("allow-reboot", rebootVar, os.Getenv(rebootVar))
	fs.Var(allowReboot, "allow-reboot", "let the owner reboot the computer from the API and the web UI: under systemd or launchd with the root helper of massimo install -reboot-helper, and as the Windows service (default "+rebootVar+", "+switchHelp+"; else off)")
	announceOn, announceErr := newSwitchFlag("announce", env.Announce, os.Getenv(env.Announce))
	fs.Var(announceOn, "announce", "announce the server on the LAN by mDNS as "+announce.ServiceType+", with its node, version and certificate, for massimo discover and the apps (default "+env.Announce+", "+switchHelp+"; else on when -listen is reachable from the LAN, off on a loopback address)")
	hstsFlag := fs.String("hsts", os.Getenv(hstsVar), "with TLS, tell browsers to use https:// for this host for this many days (Strict-Transport-Security; default "+hstsVar+", else 365); 0 sends max-age=0, which clears it, and is the way before switching TLS off")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo [-data <dir>] serve [-listen 127.0.0.1:8099] [-addr <ip>] [-proxy <addr>[:<port>]] [-log <file>] [-log-level debug] [-tls files -tls-cert <pem> -tls-key <pem> | -tls self-signed | -tls acme -tls-names <name> [-tls-http :80 | -acme-dns-hook <script>]]\n\nKeep every configured Cube connected, serve the JSON API (see internal/server\nfor the routes) and answer as a Cube on the LAN (port 62910 and UDP\ndiscovery), so that other programs and the original software connect to the\nserver instead of the Cube, which serves only one client at a time.\n\nOn an address the LAN reaches, the server announces itself by mDNS as\n_massimo._tcp (-announce), so that massimo discover and the apps find it.\n\nThe Cubes and the server's settings are kept in massimo.db in the data\ndirectory (-data, MASSIMO_DATA, else the current directory). At every start\na Cube of the settings file (MASSIMO_CUBE_<SN>_ADDRESS, _NAME, _PROXY) that\nthe store does not know is added to it; afterwards the store wins, and a\nline of the file that differs is logged.\n\nWith -tls files the API and the web UI are served over TLS from the\ncertificate and key files, re-read when they change, so a renewal needs no\nrestart; with -tls self-signed from a certificate the server makes at its\nfirst start for the box's names (and -tls-names), which browsers warn about\nonce per device: compare the fingerprint the log shows; with -tls acme from\nLet's Encrypt for a DNS name that resolves to this box, validated on port\n443 (TLS-ALPN-01), or on port 80 too with -tls-http :80 (HTTP-01), or with\n-acme-dns-hook through a script that sets the DNS record (DNS-01: no port,\nwildcards), kept and renewed in the data directory's tls/acme/. Browsers are then told to use\nhttps:// for a year (-hsts). Without -tls the listener is plain HTTP, for\nlocalhost or behind a reverse proxy.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	var logOut io.Writer = os.Stderr
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		defer f.Close()
		logOut = io.MultiWriter(os.Stderr, f)
	}
	logger, err := newLoggerAt(logOut, *logLevelFlag, "-log-level or "+env.LogLevel)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	hsts, err := hstsDays(*hstsFlag)
	if err != nil {
		return err
	}
	trusted, err := server.ParseTrustedProxies(*trustedProxies)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	restartFor, restartBlocked, err := restartBy(*restartMode, currentProcess())
	if err != nil {
		return err
	}
	managed, _, _ := restartBy(restartAuto, currentProcess())
	hup, err := sighupAction(*sighupMode, managed != "", signal.Ignored(syscall.SIGHUP))
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		hup = "none" // no SIGHUP to handle
	}
	// The settings as this run started with them, for what a change of
	// the settings file applies only at the next start (reload.go).
	startSettings := settings()
	settingsRead := time.Now() // the settings file as the start read it
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	if rebootErr != nil && !given["allow-reboot"] {
		return fmt.Errorf("serve: %w", rebootErr)
	}
	if announceErr != nil && !given["announce"] {
		return fmt.Errorf("serve: %w", announceErr)
	}
	if !announceOn.set {
		announceOn.on = !loopbackListen(*listen)
	}
	tlsCfg := serve.TLS{Mode: *tlsFlag, Cert: *tlsCert, Key: *tlsKey, Names: *tlsNames, ACMEEmail: *acmeEmail, ACMEDirectory: *acmeDirectory, HTTP: *tlsHTTP, DNSHook: *acmeDNSHook}
	tlsInEffect := serve.TLSMode(*tlsFlag, *tlsCert, *tlsKey)

	// SIGINT from a terminal, SIGTERM from systemd, launchd and kill, and
	// the Windows service manager's stop: each stops the server, which
	// writes the log's pending events, before the store is closed; so does
	// a SIGHUP that -sighup makes a stop (sighup.go).
	stopOn := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if hup == sighupStop {
		stopOn = append(stopOn, syscall.SIGHUP)
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopOn...)
	defer stop()
	ctx, cancel := serviceContext(ctx)
	defer cancel()
	// A restart from the API ends this context too (restart.go), or, as a
	// Windows service, asks the service manager to stop and start it.
	ctx, restartCancel := context.WithCancel(ctx)
	defer restartCancel()
	rs := &restarter{by: restartFor, cancel: restartCancel}
	if restartFor == byWindows {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		if rs.helper, restartBlocked = restartHelperStarter(logger, exe); rs.helper == nil {
			restartFor = ""
			logger.Warn("the restart cannot be started from the API", "reason", restartBlocked)
		}
	}
	reboot := serveReboot(logger, allowReboot.on)
	upd := server.UpdateOptions{Version: version, Channel: updateChannel(), Request: *updateRequest, Status: *updateStatus}
	if m, ok := install.Packaged(""); ok {
		upd.Managed = m.String()
	}
	if runtime.GOOS == "windows" && *updateRequest != "" {
		if upd.Start, upd.Blocked, err = serveUpdateHelper(logger, *listen, tlsInEffect != "off", *updateRequest, *updateStatus); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
	}

	cfg := serve.Config{Listen: *listen, Addr: *addr, DataDir: dataDir(), Version: versionString(), SettingsFile: env.File(), Serial: selectedSN, SerialGiven: snExplicit,
		Proxies: proxies, ProxySerial: *proxySerial, ShowHidden: showAll || showHidden, Token: *token, DiscoveryTargets: discoverTargets, DiscoveryListen: *discoverListen,
		TLS: tlsCfg, HSTSDays: hsts, TrustedProxies: trusted, Update: upd, Reboot: reboot,
		Restart: server.RestartOptions{By: restartFor, Blocked: restartBlocked, Restart: rs.restart}, LogAttrs: []any{"sighup", hup}, Logger: logger,
		Announce: serve.Announce{On: announceOn.on, Version: buildVersion()}}
	// The flags given win for this run; otherwise the store's settings.
	for _, d := range []struct {
		flag string
		v    *time.Duration
		to   **time.Duration
	}{{"poll", poll, &cfg.PollInterval}, {"refresh", refresh, &cfg.RefreshInterval}, {"discover-interval", discoverEvery, &cfg.DiscoverInterval}} {
		if given[d.flag] {
			*d.to = d.v
		}
	}
	inst, err := serve.Open(ctx, cfg)
	if err != nil {
		return err
	}
	srv := inst.Server()
	// The settings file while the server runs (reload.go): looked at every
	// few seconds, and read again on SIGHUP where that is a reload, which
	// also checkpoints the store.
	reloader := newSettingsReloader(env.File(), logger, srv, startSettings, given)
	go watchSettings(ctx, env.File(), settingsWatchEvery, func() { reloader.reload(ctx, "the settings file changed") })
	// POST /api/v1/reload: what SIGHUP does, from System settings →
	// Maintenance; the file counts as read at the start.
	srv.SetReloader(env.File(), func(rctx context.Context) (server.ReloadResult, error) {
		changed, pending, err := reloader.reload(rctx, "the API")
		if err != nil {
			return server.ReloadResult{}, err
		}
		checkpointStore(rctx, inst.Store(), logger)
		return server.ReloadResult{Changed: nonNil(changed), Pending: nonNil(pending), ReadAt: time.Now()}, nil
	})
	srv.SetSettingsRead(settingsRead)
	// GET /api/v1/config: the settings in effect, as massimo config shows
	srv.SetConfigView(func() server.ConfigView { return configView(*listen, tlsInEffect) })
	if hup == sighupReload {
		hups := make(chan os.Signal, 1)
		signal.Notify(hups, syscall.SIGHUP)
		go func() {
			defer signal.Stop(hups)
			for {
				select {
				case <-ctx.Done():
					return
				case <-hups:
					reloader.reload(ctx, "SIGHUP")
					checkpointStore(ctx, inst.Store(), logger)
				}
			}
		}()
	}
	// The store is closed when Run returns, once the server has written
	// the log's pending events.
	err = inst.Run(ctx)
	if rs.asked.Load() {
		logger.Info("stopped for a restart", "by", restartFor, "exit", exitRestart)
	}
	return rs.result(err)
}

// loopbackListen says whether the listen address is reachable from this
// box alone: a loopback address or localhost, not every interface.
func loopbackListen(listen string) bool {
	host := serve.ListenHost(listen)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// serveReboot finds who reboots this server's computer (reboot.go) and
// what asks for it; a request a helper never took is removed first.
func serveReboot(log *slog.Logger, allowed bool) server.RebootOptions {
	by, blocked, helper := rebootBy(currentProcess(), "")
	o := server.RebootOptions{By: by, Blocked: blocked, Allowed: allowed, Helper: helper}
	switch by {
	case byWindows:
		o.Reboot = windowsRebooter()
	case bySystemd, byLaunchd:
		removeStaleReboot(install.RebootRequest, log)
		o.Reboot = rebootRequester(install.RebootRequest)
	}
	return o
}

// defaultUpdateStatus is the helper's status file where install puts it:
// the Unix path, on Windows the one in %ProgramData%\Massimo.
func defaultUpdateStatus() string {
	if runtime.GOOS == "windows" {
		return install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).Status
	}
	return update.DefaultStatusPath
}

// serveUpdateHelper prepares the start of the update helper by the
// server itself (Windows): this binary in its helper mode, with the
// /healthz of the listen address, https:// when this server has TLS,
// and update.log next to the status file.
func serveUpdateHelper(log *slog.Logger, listen string, tls bool, request, status string) (func() error, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	health, err := install.HealthURL(listen, tls)
	if err != nil {
		return nil, "", err
	}
	start, blocked := updateHelperStarter(log, exe, request, status, health, filepath.Join(filepath.Dir(status), "update.log"))
	if blocked != "" {
		log.Warn("updates cannot be started from the API", "reason", blocked)
	}
	return start, blocked, nil
}
