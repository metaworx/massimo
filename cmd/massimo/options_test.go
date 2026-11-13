// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"net"
	"os"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

func TestGlobalOptionsSelectTheCube(t *testing.T) {
	t.Setenv("MASSIMO_CUBE_NAME", "default")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_NAME", "one")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_SN", "ignored-here")
	t.Setenv("MASSIMO_ENV_FILE", "")
	t.Cleanup(func() { selectedSN, snExplicit, showHidden, envGiven = "", false, false, false })
	selectedSN = ""
	rest, err := globalOptions([]string{"-env", "x.env", "--sn=keq0000001", "-show-hidden", "cube", "-addr", "y"})
	if err != nil || len(rest) != 3 || rest[0] != "cube" {
		t.Fatalf("rest %v err %v", rest, err)
	}
	if selectedSN != "keq0000001" || !snExplicit || !showHidden {
		t.Fatalf("sn %q explicit %v show %v", selectedSN, snExplicit, showHidden)
	}
	if v := cubeEnv(env.Name); v != "one" {
		t.Fatalf("name %q", v)
	}
	addr := ""
	if err := defaultAddr(&addr, "cube"); err != nil || addr != "10.0.0.1" {
		t.Fatalf("addr %q err %v", addr, err)
	}
	addr = "given"
	if err := defaultAddr(&addr, "cube"); err != nil || addr != "given" {
		t.Fatalf("given addr %q err %v", addr, err)
	}
	if l, err := hiddenDevices(); err != nil || l != nil {
		t.Fatalf("hidden with -show-hidden %v %v", l, err)
	}
	if rest, _ := globalOptions([]string{"--version"}); len(rest) != 1 || rest[0] != "--version" {
		t.Fatalf("version alias %v", rest)
	}
	selectedSN, snExplicit = "", false
	useDeviceSerial("KEQ0000001")
	if v := cubeEnv(env.Name); v != "one" {
		t.Fatalf("name after device serial %q", v)
	}
}

// -no-env reads no settings file, os.DevNull, which config says; with
// -env, in either order, it is an error.
func TestNoEnv(t *testing.T) {
	t.Setenv(env.FileVar, "from-the-environment.env")
	t.Cleanup(func() { envGiven, noEnv = false, false })
	envGiven, noEnv = false, false
	rest, err := globalOptions([]string{"-no-env", "config"})
	if err != nil || len(rest) != 1 || rest[0] != "config" {
		t.Fatalf("rest %v err %v", rest, err)
	}
	if f := env.File(); f != os.DevNull || !noEnv {
		t.Fatalf("file %q, noEnv %v", f, noEnv)
	}
	if l := envFileLine(); l != "env file:      none (-no-env)" {
		t.Fatalf("config: %q", l)
	}
	for _, args := range [][]string{{"-env", "x.env", "-no-env", "config"}, {"-no-env", "-env=x.env", "config"}} {
		envGiven, noEnv = false, false
		if _, err := globalOptions(args); err == nil || err.Error() != "-env and -no-env exclude each other" {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestDirectAndNoDirect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	dl, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := dl.Addr().String()
	dl.Close()
	t.Setenv("MASSIMO_CUBE_ADDRESS", "")
	t.Setenv("MASSIMO_CUBE_PROXY", "")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", dead)
	t.Cleanup(func() { selectedSN, snExplicit, proxyMode = "", false, env.ProxyFallback })
	selectedSN, snExplicit, proxyMode = "KEQ0000001", true, env.ProxyFallback

	// Default: the proxy does not answer, the Cube itself.
	addr := ""
	if err := defaultAddr(&addr, "backup"); err != nil || addr != "10.0.0.1" {
		t.Fatalf("fallback: addr %q err %v", addr, err)
	}
	// -no-direct: an error; -addr still wins.
	if _, err := globalOptions([]string{"-no-direct", "backup"}); err != nil || proxyMode != env.ProxyRequired {
		t.Fatalf("-no-direct: %v %v", err, proxyMode)
	}
	addr = ""
	if err := defaultAddr(&addr, "backup"); err == nil || !strings.Contains(err.Error(), "-direct") {
		t.Fatalf("-no-direct: addr %q err %v", addr, err)
	}
	addr = "given"
	if err := defaultAddr(&addr, "backup"); err != nil || addr != "given" {
		t.Fatalf("-addr wins: %q %v", addr, err)
	}
	// Both together are refused.
	if _, err := globalOptions([]string{"-direct", "backup"}); err == nil || !strings.Contains(err.Error(), "exclude") {
		t.Fatalf("-no-direct -direct: %v", err)
	}
	// -direct: the Cube itself although the proxy answers.
	proxyMode = env.ProxyFallback
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", ln.Addr().String())
	rest, err := globalOptions([]string{"-direct", "backup"})
	if err != nil || len(rest) != 1 || rest[0] != "backup" || proxyMode != env.ProxySkip {
		t.Fatalf("-direct: rest %v err %v mode %v", rest, err, proxyMode)
	}
	addr = ""
	if err := defaultAddr(&addr, "backup"); err != nil || addr != "10.0.0.1" {
		t.Fatalf("-direct: addr %q err %v", addr, err)
	}
	if _, err := globalOptions([]string{"-no-direct", "backup"}); err == nil {
		t.Fatal("-direct -no-direct accepted")
	}
}
