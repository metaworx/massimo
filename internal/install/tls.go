// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// The TLS of the installed service: -tls-cert and -tls-key are copied
// beside the settings, owned by the service user, and the settings file
// gets MASSIMO_TLS, MASSIMO_TLS_CERT and MASSIMO_TLS_KEY, which serve
// reads at its start (docs/store.md section 4) and later installs keep,
// as they keep -listen. The update helper's health URL follows: https://
// where the settings say the server has TLS.
const (
	tlsVar     = "MASSIMO_TLS"
	tlsCertVar = "MASSIMO_TLS_CERT"
	tlsKeyVar  = "MASSIMO_TLS_KEY"
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

// checkTLS refuses TLS options that cannot be installed.
func (o Options) checkTLS() error {
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
	case "self-signed", "acme":
		return fmt.Errorf("-tls %s is not in this version yet; use -tls-cert and -tls-key", o.TLS)
	default:
		return fmt.Errorf("-tls %q: want off or files", o.TLS)
	}
}

// tlsFiles installs the TLS options: the files copied to certDst and
// keyDst (system paths; owned by the service user where chown is set,
// else the directory's rights hold), and the variables set in the
// settings file env.
func (in *installer) tlsFiles(certDst, keyDst, env string, chown bool) error {
	switch in.tlsMode() {
	case "":
		return nil
	case "off":
		return in.setSettings(env, chown, [][2]string{{tlsVar, "off"}})
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
			if err := in.run("chown", in.svcUser+":"+in.svcUser, in.path(f[1])); err != nil {
				return err
			}
		}
	}
	return in.setSettings(env, chown, [][2]string{{tlsVar, "files"}, {tlsCertVar, certDst}, {tlsKeyVar, keyDst}})
}

// setSettings sets variables in the settings file env (a system path),
// keeping its mode and, where chown is set, its owner.
func (in *installer) setSettings(env string, chown bool, vars [][2]string) error {
	var names []string
	for _, v := range vars {
		names = append(names, v[0]+"="+v[1])
	}
	in.say("set %s in %s", strings.Join(names, " "), env)
	if in.DryRun {
		return nil
	}
	if err := setEnvLines(in.path(env), vars); err != nil {
		return err
	}
	if chown {
		return in.run("chown", in.svcUser+":"+in.svcUser, in.path(env))
	}
	return nil
}

// setEnvLines sets KEY=VALUE lines in a settings file, replacing the
// line of a variable that is there (an "export " prefix too) and
// appending the others, written atomically with the file's mode.
func setEnvLines(path string, vars [][2]string) error {
	mode := os.FileMode(0o600)
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(b) == 0 {
		lines = nil
	}
	for _, v := range vars {
		found := false
		for i, line := range lines {
			k, _, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
			if ok && strings.TrimSpace(k) == v[0] {
				lines[i] = v[0] + "=" + v[1]
				found = true
			}
		}
		if !found {
			lines = append(lines, v[0]+"="+v[1])
		}
	}
	return writeAtomic(path, []byte(strings.Join(lines, "\n")+"\n"), mode)
}

// tlsOn says whether settings read with readEnv make serve serve TLS.
func tlsOn(vars map[string]string) bool {
	switch vars[tlsVar] {
	case "files", "self-signed", "acme":
		return true
	case "":
		return vars[tlsCertVar] != "" || vars[tlsKeyVar] != ""
	}
	return false
}

// tlsEffective says whether the installed service has TLS: by this
// run's options, else by the settings file env (a path under Root), or
// the one being copied in a dry run.
func (in *installer) tlsEffective(env string) bool {
	switch in.tlsMode() {
	case "off":
		return false
	case "":
	default:
		return true
	}
	if _, err := os.Stat(env); err != nil {
		env = in.Settings
	}
	if env == "" {
		return false
	}
	vars, err := readEnv(env)
	if err != nil {
		return false
	}
	return tlsOn(vars)
}

// checkTLSSettings warns when the settings name TLS files serve cannot
// read.
func (in *installer) checkTLSSettings(vars map[string]string) {
	if !tlsOn(vars) || vars[tlsVar] != "" && vars[tlsVar] != "files" {
		return
	}
	for _, k := range []string{tlsCertVar, tlsKeyVar} {
		switch v := vars[k]; {
		case v == "":
			fmt.Fprintf(in.Out, "warning: no %s: serve cannot start with %s=files\n", k, tlsVar)
		default:
			if _, err := os.Stat(in.path(v)); err != nil {
				fmt.Fprintf(in.Out, "warning: %s=%s: %v; serve cannot start without it\n", k, v, err)
			}
		}
	}
}
