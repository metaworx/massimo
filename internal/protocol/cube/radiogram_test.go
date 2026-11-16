// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The radiogram types are the FrameType list of docs/cube-protocol.md,
// code for code and name for name.
func TestRadiogramTypesAreTheDocumentedList(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/cube-protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start := strings.Index(doc, "The original software's `FrameType` enum")
	end := strings.Index(doc[start:], "(read again from the jar")
	if start < 0 || end < 0 {
		t.Fatal("the FrameType list is not in docs/cube-protocol.md")
	}
	listed := map[RadiogramType]string{}
	for _, m := range regexp.MustCompile(`0x([0-9A-Fa-f]{2}) \**([A-Z][A-Za-z]+)`).FindAllStringSubmatch(doc[start:start+end], -1) {
		code, _ := strconv.ParseUint(m[1], 16, 8)
		listed[RadiogramType(code)] = m[2]
	}
	if len(listed) != 26 {
		t.Fatalf("the list has %d types, want 26: %v", len(listed), listed)
	}
	if !maps.Equal(listed, radiogramNames) {
		for _, code := range slices.Sorted(maps.Keys(listed)) {
			if radiogramNames[code] != listed[code] {
				t.Errorf("0x%02x: the docs say %q, the code %q", uint8(code), listed[code], radiogramNames[code])
			}
		}
		t.Fatalf("the code has %d types, the docs %d", len(radiogramNames), len(listed))
	}
}

// The radiogram types' categories are those docs/cube-protocol.md lists
// after the FrameType list, code for code, and none for the rest; their
// severity ranks Assignment and Remove first, then Configuration, then
// Control, then the rest (D181).
func TestRadiogramCategoriesAreTheDocumentedOnes(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/cube-protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start := strings.Index(doc, "Each type has a category")
	end := strings.Index(doc[start:], "none for the rest.")
	if start < 0 || end < 0 {
		t.Fatal("the categories are not in docs/cube-protocol.md")
	}
	names := map[string]RadiogramCategory{}
	for c, n := range radiogramCategoryNames {
		names[n] = c
	}
	listed := map[RadiogramType]RadiogramCategory{}
	text := strings.Join(strings.Fields(doc[start:start+end]), " ")
	code := regexp.MustCompile(`0x([0-9A-F]{2})(?:–0x([0-9A-F]{2}))?`)
	for _, m := range regexp.MustCompile(`([A-Z][a-z]+) for ((?:0x[0-9A-F]{2}(?:–0x[0-9A-F]{2})?(?:, | and )?)+)`).FindAllStringSubmatch(text, -1) {
		c, ok := names[m[1]]
		if !ok || c == RadiogramCategoryNone {
			t.Fatalf("the docs name a category %q", m[1])
		}
		for _, r := range code.FindAllStringSubmatch(m[2], -1) {
			from, _ := strconv.ParseUint(r[1], 16, 8)
			to := from
			if r[2] != "" {
				to, _ = strconv.ParseUint(r[2], 16, 8)
			}
			for v := from; v <= to; v++ {
				listed[RadiogramType(v)] = c
			}
		}
	}
	if len(listed) != 14 {
		t.Fatalf("the docs give %d types a category, want 14: %v", len(listed), listed)
	}
	for v := range 256 {
		r := RadiogramType(v)
		if got, want := r.Category(), listed[r]; got != want {
			t.Errorf("0x%02x: category %s, the docs %s", v, got, want)
		}
	}
	for _, c := range []struct {
		r    RadiogramType
		want int
	}{
		{RadiogramAddLinkPartner, 3}, {RadiogramRemoveGroupAddress, 3}, {RadiogramReset, 3},
		{RadiogramConfigWeekTemperatureProfile, 2}, {RadiogramConfigValveFunctions, 2}, {RadiogramSwitchDisplaySetPointActual, 2},
		{RadiogramControlMode, 1}, {RadiogramSetPointTemperature, 1}, {RadiogramLockManualControls, 1},
		{RadiogramTimeInformation, 0}, {RadiogramWakeUp, 0}, {RadiogramSetPointAndCurrentTemperature, 0}, {0xf6, 0},
	} {
		if got := c.r.Severity(); got != c.want {
			t.Errorf("%s: severity %d, want %d", c.r, got, c.want)
		}
	}
}

// The radio command codes Massimo sends are the radiogram types of the
// same names in the original software.
func TestRadioCommandCodesAreRadiogramTypes(t *testing.T) {
	for code, name := range map[byte]string{
		RFSetTemperature:  "ControlMode",
		RFSetWeekProgram:  "ConfigWeekTemperatureProfile",
		RFSetTemperatures: "ConfigTemperatures",
		RFSetValveConfig:  "ConfigValveFunctions",
		RFAddLinkPartner:  "AddLinkPartner",
		RFRemoveLink:      "RemoveLinkPartner",
		RFSetGroupAddress: "SetGroupAddress",
		RFRemoveGroup:     "RemoveGroupAddress",
		RFLockControls:    "LockManualControls",
		RFSetDisplayMode:  "SwitchDisplaySetPointActual",
	} {
		if got := RadiogramType(code).Name(); got != name {
			t.Errorf("0x%02x: %q, want %q", code, got, name)
		}
	}
	if RFSetTemperature != 0x40 || RFSetWeekProgram != 0x10 || RFLockControls != 0x80 || RFSetDisplayMode != 0x82 {
		t.Fatal("the codes moved")
	}
}

// Byte 4 of an L entry is the failed radiogram's type while the error
// flag is set, and nothing without it: entries as the owner's Cubes
// showed them (firmware 1.1.3), their addresses placeholders.
func TestDeviceStateFailedRadiogram(t *testing.T) {
	entry := func(rf byte, b4, f1 byte) []byte {
		return []byte{0x0b, 0x0a, 0x01, rf, b4, f1, 0x18, 0x00, 0x28, 0x00, 0x00, 0x00}
	}
	var l []byte
	l = append(l, entry(0x01, 0x03, 0x1a)...) // TimeInformation, valid
	l = append(l, entry(0x02, 0x21, 0x1a)...) // RemoveLinkPartner
	l = append(l, entry(0x03, 0xf1, 0x0c)...) // WakeUp, no state yet
	l = append(l, entry(0x04, 0x09, 0x12)...) // 0x09 after a restart, no flag
	l = append(l, entry(0x05, 0xf6, 0x1a)...) // a code the list lacks
	l = append(l, entry(0x06, 0x03, 0x12)...) // 0x03 kept after an r:, no flag
	types := map[RFAddress]DeviceType{}
	for i := RFAddress(0x0a0101); i <= 0x0a0106; i++ {
		types[i] = DeviceRadiatorThermostat
	}
	st, err := ParseDeviceList(b64.EncodeToString(l), types)
	if err != nil || len(st) != 6 {
		t.Fatalf("%d entries, %v", len(st), err)
	}
	for i, want := range []struct {
		code  int // -1 for none
		name  string
		json  string
		print string
	}{
		{0x03, "TimeInformation", `{"code":3,"name":"TimeInformation"}`, "TimeInformation"},
		{0x21, "RemoveLinkPartner", `{"code":33,"name":"RemoveLinkPartner"}`, "RemoveLinkPartner"},
		{0xf1, "WakeUp", `{"code":241,"name":"WakeUp"}`, "WakeUp"},
		{-1, "", "", ""},
		{0xf6, "", `{"code":246}`, "0xf6"},
		{-1, "", "", ""},
	} {
		s := st[i]
		raw, _ := json.Marshal(s)
		var obj map[string]json.RawMessage
		json.Unmarshal(raw, &obj)
		if want.code < 0 {
			if s.Error || s.FailedRadiogram != nil || obj["failed_radiogram"] != nil {
				t.Errorf("%s: no flag, yet %v %v", s.RFAddress, s.Error, s.FailedRadiogram)
			}
			continue
		}
		r := s.FailedRadiogram
		if !s.Error || r == nil || int(*r) != want.code || r.Name() != want.name || r.String() != want.print {
			t.Errorf("%s: error %v, failed %v", s.RFAddress, s.Error, r)
			continue
		}
		if got := string(obj["failed_radiogram"]); got != want.json {
			t.Errorf("%s: failed_radiogram %s, want %s", s.RFAddress, got, want.json)
		}
		var back RadiogramType
		if err := json.Unmarshal(obj["failed_radiogram"], &back); err != nil || back != *r {
			t.Errorf("%s: read back as %v, %v", s.RFAddress, back, err)
		}
	}
	var bare RadiogramType
	if err := json.Unmarshal([]byte("33"), &bare); err != nil || bare != RadiogramRemoveLinkPartner {
		t.Errorf("a bare code: %v %v", bare, err)
	}
	if err := json.Unmarshal([]byte(`"x"`), &bare); err == nil {
		t.Error("a string read as a radiogram type")
	}
}
