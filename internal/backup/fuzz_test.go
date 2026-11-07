// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package backup

// A backup file comes from an upload (a restore from a file): its parser
// may not panic, and a file it reads writes out as it reads back.
// `go test` runs the seeds; `make fuzz` runs the target for FUZZTIME.

import (
	"bytes"
	"testing"
)

func FuzzUnmarshal(f *testing.F) {
	for _, yamlFormat := range []bool{false, true} {
		b, err := (&File{Version: Version, Tool: "massimo test"}).Marshal(yamlFormat)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b, yamlFormat)
	}
	f.Add([]byte(`{"version":1,"cube":{"serial":"KEQ0000100"},"devices":[{"rf_address":"0a010d"}]}`), false)
	f.Add([]byte("version: 1\ncube:\n  serial: KEQ0000100\n"), true)
	f.Add([]byte(`{"version":99}`), false)
	f.Fuzz(func(t *testing.T, data []byte, yamlFormat bool) {
		file, err := Unmarshal(data, yamlFormat)
		if err != nil {
			return
		}
		out, err := file.Marshal(yamlFormat)
		if err != nil {
			t.Fatalf("read, but does not write: %v", err)
		}
		again, err := Unmarshal(out, yamlFormat)
		if err != nil {
			t.Fatalf("written, but does not read back: %v\n%s", err, out)
		}
		out2, err := again.Marshal(yamlFormat)
		if err != nil || !bytes.Equal(out, out2) {
			t.Fatalf("written twice differently (%v):\n%s\n%s", err, out, out2)
		}
	})
}
