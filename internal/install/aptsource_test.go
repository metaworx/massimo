// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The snippet of packaging/apt/publish.sh, with its comment.
const aptSnippet = `# The Massimo apt repository (https://massimo.metaworx.ch,
# packaging/README.md, "The apt repository"): one suite per update channel (alpha); the key comes with the
# package massimo-archive-keyring.
Types: deb
URIs: https://massimo.metaworx.ch/apt
Suites: alpha
Components: main
Signed-By: /usr/share/keyrings/massimo-archive-keyring.gpg
`

// TestAptSwitch: the suite and Signed-By change, every other line stays,
// and a rolling suite names the rolling keyring; back again gives the
// snippet as it was.
func TestAptSwitch(t *testing.T) {
	devel, err := AptSwitch([]byte(aptSnippet), "devel", true)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.NewReplacer("Suites: alpha", "Suites: devel", AptKeyring, AptRollingKeyring).Replace(aptSnippet)
	if string(devel) != want {
		t.Fatalf("to devel:\n%s\nwant\n%s", devel, want)
	}
	if s, err := AptSuite(writeTemp(t, devel)); err != nil || s != "devel" {
		t.Fatalf("AptSuite after the switch = %q, %v", s, err)
	}
	back, err := AptSwitch(devel, "alpha", false)
	if err != nil || string(back) != aptSnippet {
		t.Fatalf("back to alpha: %v\n%s", err, back)
	}
	// A local repository, an old address, a field name of another case.
	for _, src := range []string{
		strings.Replace(aptSnippet, "https://massimo.metaworx.ch/apt", "file:///srv/repo", 1),
		strings.Replace(aptSnippet, "https://massimo.metaworx.ch/apt", "https://metaworx.github.io/massimo/apt/", 1),
		strings.Replace(aptSnippet, "Suites:", "suites:", 1),
	} {
		if _, err := AptSwitch([]byte(src), "beta", false); err != nil {
			t.Errorf("%v for\n%s", err, src)
		}
	}
}

// TestAptSwitchLeavesOthersAlone: a source the README did not set up is
// its owner's.
func TestAptSwitchLeavesOthersAlone(t *testing.T) {
	for want, src := range map[string]string{
		"2 suites":                strings.Replace(aptSnippet, "Suites: alpha", "Suites: alpha beta", 1),
		"not the project's":       strings.Replace(aptSnippet, "https://massimo.metaworx.ch/apt", "https://mirror.example.invalid/apt", 1),
		"Signed-By is not a file": strings.Replace(aptSnippet, AptKeyring, "/etc/apt/keyrings/mine.gpg", 1),
		"no signed-by line":       strings.Replace(aptSnippet, "Signed-By: "+AptKeyring+"\n", "", 1),
		"more than one entry":     aptSnippet + "\nTypes: deb\nURIs: https://deb.example.invalid\nSuites: x\n",
		"names suites twice":      aptSnippet + "Suites: beta\n",
		"its signed-by line goes": strings.Replace(aptSnippet, "Signed-By: "+AptKeyring, "Signed-By:\n -----BEGIN PGP PUBLIC KEY BLOCK-----", 1),
	} {
		if _, err := AptSwitch([]byte(src), "beta", false); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v for\n%s", want, err, src)
		}
	}
}

// writeTemp writes b to a file of its own and returns its path.
func writeTemp(t *testing.T, b []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "massimo.sources")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
