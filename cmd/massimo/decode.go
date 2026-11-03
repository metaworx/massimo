// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// decoded is the JSON document produced by "massimo decode".
type decoded struct {
	Hello      *cube.Hello          `json:"hello,omitempty"`
	Metadata   *cube.Metadata       `json:"metadata,omitempty"`
	Configs    []*cube.DeviceConfig `json:"configs,omitempty"`
	Devices    []cube.DeviceState   `json:"devices,omitempty"`
	SendResult *cube.SendResult     `json:"send_result,omitempty"`
	NewDevices []*cube.NewDevice    `json:"new_devices,omitempty"`
	NTP        []string             `json:"ntp,omitempty"`
	Other      []string             `json:"other,omitempty"`
	Errors     []string             `json:"errors,omitempty"`
}

func runDecode(args []string) error {
	var in io.Reader = os.Stdin
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	raw, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	msgs, err := cube.SplitLines(string(raw))
	if err != nil {
		return err
	}
	out := decode(msgs)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func decode(msgs []cube.Message) *decoded {
	out := &decoded{}
	var parts []cube.MetadataPart
	types := map[cube.RFAddress]cube.DeviceType{}
	fail := func(m cube.Message, err error) {
		out.Errors = append(out.Errors, fmt.Sprintf("%c: %v", m.Type, err))
	}
	for _, m := range msgs {
		switch m.Type {
		case 'H':
			h, err := cube.ParseHello(m.Data)
			if err != nil {
				fail(m, err)
				continue
			}
			out.Hello = h
		case 'M':
			p, err := cube.ParseMetadataPart(m.Data)
			if err != nil {
				fail(m, err)
				continue
			}
			parts = append(parts, p)
			if len(parts) == p.Count {
				md, err := cube.ParseMetadata(parts)
				if err != nil {
					fail(m, err)
					continue
				}
				out.Metadata = md
				for _, d := range md.Devices {
					types[d.RFAddress] = d.Type
				}
			}
		case 'C':
			c, err := cube.ParseConfig(m.Data)
			if err != nil {
				fail(m, err)
				continue
			}
			types[c.RFAddress] = c.Type
			out.Configs = append(out.Configs, c)
		case 'L':
			st, err := cube.ParseDeviceList(m.Data, types)
			if err != nil {
				fail(m, err)
			}
			out.Devices = append(out.Devices, st...)
		case 'S':
			r, err := cube.ParseSendResult(m.Data)
			if err != nil {
				fail(m, err)
				continue
			}
			out.SendResult = r
		case 'N':
			d, err := cube.ParseNewDevice(m.Data)
			if err != nil {
				fail(m, err)
				continue
			}
			out.NewDevices = append(out.NewDevices, d)
		case 'F':
			out.NTP = cube.ParseNTPServers(m.Data)
		default:
			out.Other = append(out.Other, m.String())
		}
	}
	return out
}
