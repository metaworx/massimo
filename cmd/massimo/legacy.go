// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Lines of the original software's output that tell what it does on the
// Cube: API calls, gateway calls, radiograms, send results, update steps.
var legacyKeep = regexp.MustCompile(`MaxLocalApi:method|CubeGateway:method|ClientCommand:radiogram|DeviceSendCommand:parseStatus|CubeUpdateService|CubeUdpProxy|NewDeviceSerializer|ERROR|Exception$`)

// runLegacy starts the original MAX! software and shows only the lines
// that matter for protocol work. The complete output can be saved.
func runLegacy(args []string) error {
	fs := flag.NewFlagSet("legacy", flag.ContinueOnError)
	java := fs.String("java", "java", "java executable")
	dir := fs.String("dir", defaultLegacyDir(), "installation directory of the MAX! software (holds MaxLocalApp.jar and lib/)")
	out := fs.String("out", "", "save the complete, unfiltered output to this file")
	all := fs.Bool("all", false, "print everything instead of the filtered lines")
	width := fs.Int("width", 300, "shorten printed lines to this many characters (0 = no limit)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo legacy [-java PATH] [-dir DIR] [-out FILE] [-all] [-width N]\n\nStart the original MAX! software (needs a Java 8 runtime) and show its\nprotocol-relevant output: API and gateway calls, radiograms, send results.\nStack traces and framework noise are dropped unless -all is given.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	jar := filepath.Join(*dir, "MaxLocalApp.jar")
	if _, err := os.Stat(jar); err != nil {
		return fmt.Errorf("legacy: %s not found; give the installation directory with -dir", jar)
	}
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	cmd := exec.Command(*java, "-cp", "MaxLocalApp.jar"+sep+filepath.Join("lib", "*"), "de.eq3.max.MaxLocalAppStarter.Application")
	cmd.Dir = *dir
	cmd.Stdin = nil
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("legacy: start java: %w", err)
	}
	var save io.WriteCloser
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		save = f
		defer f.Close()
	}
	fmt.Fprintf(os.Stderr, "legacy: started %s (pid %d) in %s\n", *java, cmd.Process.Pid, *dir)
	go func() {
		cmd.Wait()
		pw.Close()
	}()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Text()
		if save != nil {
			fmt.Fprintln(save, line)
		}
		if !*all && !legacyKeep.MatchString(line) {
			continue
		}
		lan := derivedLANLine(line)
		if *width > 0 && len(line) > *width {
			line = line[:*width] + " …(" + fmt.Sprint(len(sc.Text())) + " chars)"
		}
		fmt.Println(strings.TrimRight(line, "\r"))
		if lan != "" {
			fmt.Println("    lan (derived) > " + lan)
		}
	}
	if ps := cmd.ProcessState; ps != nil && !ps.Success() {
		return fmt.Errorf("legacy: %s", ps)
	}
	return nil
}

func defaultLegacyDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramData"), "MAX")
	}
	return "."
}

// The software logs its gateway calls with arguments but never the LAN
// lines it writes. The encodings are known, so the line is derived from
// the call and printed beneath it, marked as derived.
var (
	reRadiogram = regexp.MustCompile(`commitDeviceRadiogram\(DeviceRadiogram\(opCode=([a-z]):, payload=\[([-0-9,]+)\]`)
	reDelete    = regexp.MustCompile(`deleteDevices\(\[([0-9, ]+)\],(true|false)\)`)
	reMetadata  = regexp.MustCompile(`sendMetaData\(([A-Za-z0-9+/=]+)`)
	reWakeup    = regexp.MustCompile(`wakeUpRoom\(([0-9]+)\)`)
	reConfig    = regexp.MustCompile(`getConfiguration\(([0-9]+)\)`)
	reCall      = regexp.MustCompile(`CubeGateway:methodCalled\] \(\) --> ([A-Za-z]+)\(`)
)

func derivedLANLine(line string) string {
	if m := reRadiogram.FindStringSubmatch(line); m != nil {
		var raw []byte
		for _, f := range strings.Split(m[2], ",") {
			var v int
			fmt.Sscanf(strings.TrimSpace(f), "%d", &v)
			raw = append(raw, byte(v))
		}
		return m[1] + ":" + base64.StdEncoding.EncodeToString(raw)
	}
	if m := reDelete.FindStringSubmatch(line); m != nil {
		var addrs []cube.RFAddress
		for _, f := range strings.Split(m[1], ",") {
			var v uint32
			fmt.Sscanf(strings.TrimSpace(f), "%d", &v)
			addrs = append(addrs, cube.RFAddress(v))
		}
		return cube.CmdDeleteDevices(m[2] == "true", addrs...).String()
	}
	if m := reMetadata.FindStringSubmatch(line); m != nil {
		return "m:00," + m[1]
	}
	if m := reWakeup.FindStringSubmatch(line); m != nil {
		var room int
		fmt.Sscanf(m[1], "%d", &room)
		return cube.CmdWakeup(30, cube.WakeupRoom, fmt.Sprintf("%02x", room)).String()
	}
	if m := reConfig.FindStringSubmatch(line); m != nil {
		var v uint32
		fmt.Sscanf(m[1], "%d", &v)
		return "c:" + cube.RFAddress(v).String()
	}
	if m := reCall.FindStringSubmatch(line); m != nil {
		switch m[1] {
		case "getDeviceList":
			return "l:"
		case "fetchNewDevices":
			return "n: (inclusion mode, if not already on)"
		case "cancelInclusionMode":
			return "x:"
		case "disconnect":
			return "q:"
		}
	}
	return ""
}
