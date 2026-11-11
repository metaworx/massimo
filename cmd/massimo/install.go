// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"go.metaworx.ch/massimo/internal/install"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	settings := fs.String("settings", "", "copy this settings file (a .env) as the service's settings when there are none yet")
	noHelper := fs.Bool("no-update-helper", false, "leave out the update helper (Linux and macOS: its units or LaunchDaemon; Windows: the service's -update-request; the Debian package: its path unit disabled); the web UI then cannot start updates")
	rebootHelper := fs.Bool("reboot-helper", false, "Linux and macOS: install the root helper that reboots the computer when the web UI asks (massimo-reboot.path and .service, the LaunchDaemon "+install.RebootLabel+"); the server offers the reboot once MASSIMO_REBOOT=on is in its settings; kept by later installs")
	noRebootHelper := fs.Bool("no-reboot-helper", false, "take the reboot helper out again")
	noStart := fs.Bool("no-start", false, "enable the service without starting it")
	listen := fs.String("listen", "", "serve the API and web UI on this address, such as :80 (default :8099; kept by later installs)")
	tlsMode := fs.String("tls", "", "serve over TLS: files for -tls-cert and -tls-key, self-signed for a certificate the server makes, or off; written into the settings and kept by later installs (default: what the settings say)")
	tlsCert := fs.String("tls-cert", "", "the certificate to install beside the settings for -tls files: PEM, the chain after the server's certificate")
	tlsKey := fs.String("tls-key", "", "its private key, PEM")
	tlsNames := fs.String("tls-names", "", "with -tls self-signed, names and addresses the certificate holds besides the box's own; with -tls acme, the DNS name the certificate is for; comma-separated")
	acmeEmail := fs.String("acme-email", "", "with -tls acme, the account's contact address")
	acmeDirectory := fs.String("acme-directory", "", "with -tls acme, the CA's directory URL, such as Let's Encrypt's staging one, https://acme-staging-v02.api.letsencrypt.org/directory, for a first try (default: Let's Encrypt's)")
	tlsHTTP := fs.String("tls-http", "", "with -tls acme, also listen on this plain address, such as :80, for the HTTP-01 challenge and the redirect to https://<name>:443/, the port a router forwards")
	acmeDNSHook := fs.String("acme-dns-hook", "", "with -tls acme, get the certificate by the DNS-01 challenge through this executable (packaging/acme-dns-hooks/), readable and executable by the service user; no port, wildcards allowed")
	dryRun := fs.Bool("dry-run", false, "only show what would be done")
	remove := fs.Bool("remove", false, "stop and remove the service and the binary")
	purge := fs.Bool("purge", false, "with -remove: also remove the settings and the logs (Linux and macOS: the state and the user too)")
	root := fs.String("root", "", "Linux and macOS: install below this directory instead of / and run no commands (for images and tests)")
	fs.Usage = func() {
		win := install.WindowsPaths("", "")
		fmt.Fprintf(os.Stderr, `usage: massimo install [-settings <.env>] [-listen <addr>] [-tls-cert <pem> -tls-key <pem> | -tls self-signed [-tls-names <list>] | -tls acme -tls-names <name> [-acme-email <addr>] [-acme-directory <url>] [-tls-http :80 | -acme-dns-hook <script>]] [-no-update-helper] [-reboot-helper | -no-reboot-helper] [-no-start] [-dry-run]
       massimo install -remove [-purge]

Install this binary and massimo serve as a service, or update both when
run again (existing settings are kept). With -tls-cert and -tls-key the
files are copied beside the settings, owned by the service user, and
MASSIMO_TLS, MASSIMO_TLS_CERT and MASSIMO_TLS_KEY are set in the settings,
so the service serves https:// (and the update helper checks it there);
-tls self-signed has the server make a certificate for the box's names
at its start, which browsers warn about once per device (compare the
fingerprint of the log, or of massimo tls show); -tls acme has it get
one from Let's Encrypt for a DNS name that resolves to this box, with
the router forwarding port 443 to the service (-tls-http :80 adds the
HTTP-01 challenge on port 80; -acme-dns-hook <script> takes the DNS-01
challenge instead, with no port and wildcards); -tls off switches TLS
off again. Run the service once with -hsts 0 over TLS first, or browsers
refuse http:// for a year.

- Linux with systemd: the binary as %s, the user _massimo, %s,
  the units massimo.service and the update helper
  massimo-update.path/.service, with -reboot-helper the reboot helper
  massimo-reboot.path/.service, and the log rotation; a unit edited by
  hand is kept as .bak. On a box with the Debian package only the
  settings, the listen address and the package's update and reboot
  helpers, enabled or disabled. Needs root.
- Windows: the binary as %s, settings and log in %s
  (SYSTEM and Administrators only), the service "massimo" (automatic
  start, restarted on failure) and a firewall rule for private and
  domain networks; the server starts the update helper itself when the
  web UI asks for an update, and reboots the computer itself, needing
  no helper. Needs an administrator prompt.
- macOS: the binary as %s, the hidden user %s, %s, the
  LaunchDaemons %s and the update helper %s, with
  -reboot-helper the reboot helper %s, in %s, loaded with
  launchctl. Needs root.

See packaging/README.md.

`, install.BinPath, install.EnvPath, win.Bin, win.DataDir,
			install.BinPath, install.DarwinUser, install.EnvPath, install.Label, install.HelperLabel, install.RebootLabel, install.DaemonDir)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *purge && !*remove {
		return errors.New("-purge goes with -remove")
	}
	if *rebootHelper && *noRebootHelper {
		return errors.New("-reboot-helper and -no-reboot-helper exclude each other")
	}
	o := install.Options{Root: *root, Settings: *settings, NoUpdateHelper: *noHelper, RebootHelper: *rebootHelper, NoRebootHelper: *noRebootHelper, NoStart: *noStart, DryRun: *dryRun, Listen: *listen, TLS: *tlsMode, TLSCert: *tlsCert, TLSKey: *tlsKey, TLSNames: *tlsNames, ACMEEmail: *acmeEmail, ACMEDirectory: *acmeDirectory, TLSHTTP: *tlsHTTP, ACMEDNSHook: *acmeDNSHook}
	switch runtime.GOOS {
	case "linux":
		if *remove {
			return install.Remove(o, *purge)
		}
		return install.Install(o)
	case "windows":
		if *root != "" {
			return errors.New("-root is for Linux and macOS")
		}
		if *remove {
			return install.RemoveWindows(o, *purge)
		}
		if *rebootHelper {
			fmt.Printf("Windows needs no reboot helper: the service reboots the computer itself once MASSIMO_REBOOT=on is in %s\n", install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).Env)
		}
		return install.InstallWindows(o)
	case "darwin":
		if *remove {
			return install.RemoveDarwin(o, *purge)
		}
		return install.InstallDarwin(o)
	default:
		return fmt.Errorf("install does not support %s", runtime.GOOS)
	}
}
