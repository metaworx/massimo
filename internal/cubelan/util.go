// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func joinFields(f []string) string { return strings.Join(f, ",") }

func cutComma(s string) (string, string, bool) { return strings.Cut(s, ",") }

func twoHex(n int) string { return fmt.Sprintf("%02x", n) }

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
