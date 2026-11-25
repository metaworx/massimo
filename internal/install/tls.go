// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strings"

	"go.metaworx.ch/massimo/internal/env"
)

// The TLS of the installed service: -tls-cert and -tls-key are copied
// beside the settings, owned by the service user, and the settings file
// gets MASSIMO_TLS, MASSIMO_TLS_CERT and MASSIMO_TLS_KEY, which serve
// reads at its start (docs/store.md section 4) and later installs keep,
// as they keep -listen. The update helper's health URL follows: https://
// where the settings say the server has TLS.
const (
	// The variables serve reads, named once in internal/env.
	tlsVar        = env.TLS
	tlsCertVar    = env.TLSCert
	tlsKeyVar     = env.TLSKey
	tlsNamesVar   = env.TLSNames
	acmeEmail     = env.ACMEEmail
	acmeDirectory = env.ACMEDirectory
	tlsHTTPVar    = env.TLSHTTP
	acmeDNSHook   = env.ACMEDNSHook
	// The owner's files beside the settings.
	certName = "cert.pem"
	keyName  = "key.pem"
)

// tlsMode is Options.TLS as it applies: "files" when only the files are
// given, "" when nothing is.
func (o Options) tlsMode() string {
	if o.TLS == "" && (o.TLSCert != "" || o.TLSKey != "") {
		return "files"
	}
	return o.TLS
}

// checkTLS refuses TLS options that cannot be installed: those that do
// not go together, or name a file that is not there, the hook of
// -acme-dns-hook where the service svc runs it (serviceStart.path). What
// serve's start refuses of them is checkSettingsFile's.
func (o Options) checkTLS(svc serviceStart) error {
	switch o.tlsMode() {
	case "":
		return nil
	case "off":
		if o.TLSCert != "" || o.TLSKey != "" {
			return errors.New("-tls off and -tls-cert or -tls-key exclude each other")
		}
		return nil
	case "files":
		if o.TLSCert == "" || o.TLSKey == "" {
			return errors.New("-tls files needs -tls-cert and -tls-key")
		}
		for _, p := range []string{o.TLSCert, o.TLSKey} {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("-tls: %v", err)
			}
		}
		return nil
	case "self-signed":
		if o.TLSCert != "" || o.TLSKey != "" {
			return errors.New("-tls self-signed and -tls-cert or -tls-key exclude each other")
		}
		return nil
	case "acme":
		if o.TLSCert != "" || o.TLSKey != "" {
			return errors.New("-tls acme and -tls-cert or -tls-key exclude each other")
		}
		if o.TLSNames == "" {
			return errors.New("-tls acme needs -tls-names, the DNS name the certificate is for")
		}
		if o.ACMEDirectory != "" {
			if u, err := url.Parse(o.ACMEDirectory); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
				return fmt.Errorf("-acme-directory %q: want the directory's URL, such as https://acme-staging-v02.api.letsencrypt.org/directory", o.ACMEDirectory)
			}
		}
		if o.TLSHTTP != "" {
			if _, _, err := readListen(o.TLSHTTP); err != nil {
				return fmt.Errorf("-tls-http %q: %w", o.TLSHTTP, err)
			}
		}
		if o.ACMEDNSHook != "" {
			if o.TLSHTTP != "" {
				return errors.New("-tls-http is for the HTTP-01 challenge; with -acme-dns-hook the DNS-01 challenge needs no port")
			}
			hook, ok := svc.path(o.Root, o.ACMEDNSHook)
			if !ok {
				hook = o.ACMEDNSHook
			}
			fi, err := os.Stat(hook)
			if err != nil {
				return fmt.Errorf("-acme-dns-hook: %v", err)
			}
			if runtime.GOOS != "windows" && fi.Mode()&0o111 == 0 {
				return fmt.Errorf("-acme-dns-hook %s is not executable", o.ACMEDNSHook)
			}
		}
		return nil
	default:
		return fmt.Errorf("-tls %q: want off, files, self-signed or acme", o.TLS)
	}
}

// modeVars are the TLS variables an install's -tls sets; it removes the
// others, so that a variable of the mode before (-tls-http, a DNS hook,
// which serve refuses outside acme) does not outlive a switch.
var modeVars = []string{tlsVar, tlsCertVar, tlsKeyVar, tlsNamesVar, acmeEmail, acmeDirectory, tlsHTTPVar, acmeDNSHook}

// tlsSettings are the TLS variables the -tls options set in the
// settings file, files mode's naming cert and key; none where -tls keeps
// what the file says. The others of modeVars go from the file.
func (o Options) tlsSettings(cert, key string) [][2]string {
	switch o.tlsMode() {
	case "off":
		return [][2]string{{tlsVar, "off"}}
	case "self-signed":
		// The server makes the pair at its start, as the service user,
		// into the data directory, which it may write.
		vars := [][2]string{{tlsVar, "self-signed"}}
		if o.TLSNames != "" {
			vars = append(vars, [2]string{tlsNamesVar, o.TLSNames})
		}
		return vars
	case "acme":
		// The server orders the certificate at its start, into the data
		// directory's tls/acme/.
		vars := [][2]string{{tlsVar, "acme"}, {tlsNamesVar, o.TLSNames}}
		if o.ACMEEmail != "" {
			vars = append(vars, [2]string{acmeEmail, o.ACMEEmail})
		}
		if o.ACMEDirectory != "" {
			vars = append(vars, [2]string{acmeDirectory, o.ACMEDirectory})
		}
		if o.TLSHTTP != "" {
			vars = append(vars, [2]string{tlsHTTPVar, o.TLSHTTP})
		}
		if o.ACMEDNSHook != "" {
			vars = append(vars, [2]string{acmeDNSHook, o.ACMEDNSHook})
		}
		return vars
	case "files":
		return [][2]string{{tlsVar, "files"}, {tlsCertVar, cert}, {tlsKeyVar, key}}
	}
	return nil
}

// tlsFiles installs the TLS options: the files copied to certDst and
// keyDst (system paths; owned by the service user where chown is set,
// else the directory's rights hold), and the variables set in the
// settings file env (tlsSettings), those of modeVars the mode does not
// set removed.
func (in *installer) tlsFiles(certDst, keyDst, env string, chown bool) error {
	switch in.tlsMode() {
	case "":
		return nil
	case "files": // the files first, below
	default:
		return in.setSettings(env, chown, in.tlsSettings(certDst, keyDst))
	}
	for _, f := range [][2]string{{in.TLSCert, certDst}, {in.TLSKey, keyDst}} {
		in.say("copy %s to %s", f[0], f[1])
		if !in.DryRun {
			b, err := os.ReadFile(f[0])
			if err != nil {
				return err
			}
			if err := writeAtomic(in.path(f[1]), b, 0o600); err != nil {
				return err
			}
		}
		if chown {
			if err := in.run("chown", "-h", in.svcUser+":"+in.svcUser, in.path(f[1])); err != nil {
				return err
			}
		}
	}
	return in.setSettings(env, chown, in.tlsSettings(certDst, keyDst))
}

// setSettings sets the TLS variables of a mode in the settings file env
// (a system path), removing the other modeVars, and keeps the file's
// mode and, where chown is set, its owner.
func (in *installer) setSettings(env string, chown bool, vars [][2]string) error {
	var names []string
	set := map[string]bool{}
	for _, v := range vars {
		names = append(names, v[0]+"="+v[1])
		set[v[0]] = true
	}
	var drop []string
	for _, k := range modeVars {
		if !set[k] {
			drop = append(drop, k)
		}
	}
	in.say("set %s in %s", strings.Join(names, " "), env)
	if in.DryRun {
		return nil
	}
	if err := setEnvLines(in.path(env), vars, drop); err != nil {
		return err
	}
	if chown {
		return in.run("chown", "-h", in.svcUser+":"+in.svcUser, in.path(env))
	}
	return nil
}

// utf8BOM is the UTF-8 byte order mark, which Windows' editors and
// PowerShell write at the start of a file, and serve skips there.
const utf8BOM = "\xEF\xBB\xBF"

// setEnvLines sets KEY=VALUE lines in a settings file, replacing every
// line of a variable that is there (an "export " prefix too), so the
// first, which serve takes, and appending the others, removes the lines
// of the variables in drop, and writes the file atomically with its
// mode. A name is matched by its key, as serve reads it (env.NameKey):
// on Windows a line of the name in another case is replaced, which the
// start would take before a line appended below it. A UTF-8 byte order
// mark at the file's start is skipped, as serve skips it, so that the
// first line's name is matched, and kept.
func setEnvLines(path string, vars [][2]string, drop []string) error {
	mode := os.FileMode(0o600)
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	text, bom := strings.CutPrefix(string(b), utf8BOM)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}
	if len(drop) > 0 {
		kept := lines[:0]
		for _, line := range lines {
			k, _, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
			if ok && slices.ContainsFunc(drop, func(d string) bool { return env.NameKey(d) == env.NameKey(strings.TrimSpace(k)) }) {
				continue
			}
			kept = append(kept, line)
		}
		lines = kept
	}
	for _, v := range vars {
		found := false
		for i, line := range lines {
			k, _, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
			if ok && env.NameKey(strings.TrimSpace(k)) == env.NameKey(v[0]) {
				lines[i] = v[0] + "=" + v[1]
				found = true
			}
		}
		if !found {
			lines = append(lines, v[0]+"="+v[1])
		}
	}
	text = strings.Join(lines, "\n") + "\n"
	if bom {
		text = utf8BOM + text
	}
	return writeAtomic(path, []byte(text), mode)
}

// tlsOn says whether settings read with readSettings make serve serve TLS.
func tlsOn(vars map[string]string) bool {
	switch vars[tlsVar] {
	case "files", "self-signed", "acme":
		return true
	case "":
		return vars[tlsCertVar] != "" || vars[tlsKeyVar] != ""
	}
	return false
}

// settingsVars reads the settings file env (a path under Root), or the
// one being copied in a dry run (checkedFile); nothing when there is
// neither.
func (in *installer) settingsVars(env string) map[string]string {
	env = in.checkedFile(env)
	if env == "" {
		return nil
	}
	vars, err := in.readSettings(env)
	if err != nil {
		return nil
	}
	return vars
}

// tlsEffective says whether the installed service has TLS: by this
// run's options, else by the settings file.
func (in *installer) tlsEffective(env string) bool {
	switch in.tlsMode() {
	case "off":
		return false
	case "":
		return tlsOn(in.settingsVars(env))
	}
	return true
}

// tlsHTTPEffective is the plain helper address of the installed
// service, from this run's options, else the settings file; "" without
// TLS.
func (in *installer) tlsHTTPEffective(env string) string {
	if in.TLSHTTP != "" {
		return in.TLSHTTP
	}
	if in.tlsMode() != "" && in.tlsMode() != "acme" || !in.tlsEffective(env) {
		return ""
	}
	return in.settingsVars(env)[tlsHTTPVar]
}
