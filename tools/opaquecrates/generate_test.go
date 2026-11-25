// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// entry is a file of a made-up archive: its path below the crate's
// directory, its content, for a link where it points, and whether it is
// a directory.
type entry struct {
	path, text, link string
	dir              bool
}

// archive is a made-up crate's archive as crates.io serves one: a tar
// in gzip, its entries under <name>-<version>/ unless one names a path
// of its own (outside, from /).
func archive(t *testing.T, name, version string, entries ...entry) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	tw := tar.NewWriter(z)
	for _, e := range entries {
		p := name + "-" + version + "/" + e.path
		if rest, ok := strings.CutPrefix(e.path, "/"); ok {
			p = rest
		}
		h := &tar.Header{Name: p, Mode: 0o644, Size: int64(len(e.text)), Typeflag: tar.TypeReg}
		switch {
		case e.link != "":
			h = &tar.Header{Name: p, Mode: 0o777, Linkname: e.link, Typeflag: tar.TypeSymlink}
		case e.dir:
			h = &tar.Header{Name: p, Mode: 0o755, Typeflag: tar.TypeDir}
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.text)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// manifest is a Cargo.toml as cargo publish writes it into an archive.
func manifest(name, version, licence, file string) entry {
	text := "[package]\nedition = \"2021\"\nname = \"" + name + "\"\nversion = \"" + version + "\"\n"
	if licence != "" {
		text += "license = \"" + licence + "\"\n"
	}
	if file != "" {
		text += "license-file = '" + file + "'\n"
	}
	return entry{path: "Cargo.toml", text: text + "\n[dependencies.serde]\nversion = \"1\"\nlicense = \"not the package's\"\n"}
}

// described is a Cargo.toml with a description of several lines after
// its [package] table's keys, which holds a licence and a table of its
// own, as text.
func described(m entry) entry {
	m.text = strings.Replace(m.text, "\n\n[dependencies.serde]", "\ndescription = \"\"\"\nNot the [package]'s:\n[package]\nlicense = \"WTFPL\"\n\"\"\"\n\n[dependencies.serde]", 1)
	return m
}

// made is a made-up index in a temporary directory: each crate's archive
// as <name>-<version>.crate, and the file whose head lists them by their
// SHA-256 (or by the one sums gives), with an old body after it.
func made(t *testing.T, head string, sums map[string]string, crates map[string][]byte, order ...string) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	var list strings.Builder
	for _, k := range order {
		sum, ok := sums[k]
		if !ok {
			sum = fmt.Sprintf("%x", sha256.Sum256(crates[k]))
		}
		fmt.Fprintf(&list, "%s %s\n", k, sum)
		if err := os.WriteFile(filepath.Join(dir, strings.ReplaceAll(k, " ", "-")+".crate"), crates[k], 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path = filepath.Join(dir, "opaque.LICENSE-3RD-PARTY")
	if err := os.WriteFile(path, []byte(head+list.String()+"\n==> old 0.0.1 (MIT) <==\n--- LICENSE\nan old text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// TestFileWritten checks the file the program writes from a made-up
// index: the head kept as it is; each crate in the list's order with the
// licence its Cargo.toml declares, not a dependency's nor one inside a
// description of several lines; at its root, and only there, the files
// named LICENSE*, LICENCE*, COPYING*, NOTICE* or UNLICENSE* in any case,
// by name, and the license-file its Cargo.toml names last; CRLF as
// LF and a text without a newline at its end given one; a text the same
// as one above, byte for byte once its line ends are LF, folded into a
// line that names that one, and a text that differs by a space not; a
// crate without a licence file saying so with its declared licence. A
// second run finds the file current, and -check does too.
func TestFileWritten(t *testing.T) {
	const mit = "MIT License\n\nPermission is hereby granted.\n"
	crates := map[string][]byte{
		"alpha 1.0.0": archive(t, "alpha", "1.0.0",
			manifest("alpha", "1.0.0", "MIT OR Apache-2.0", ""),
			entry{path: "LICENSE-MIT", text: strings.ReplaceAll(mit, "\n", "\r\n")},
			entry{path: "LICENSE-APACHE", text: "Apache License\n"},
			entry{path: "src/LICENSE", text: "not at the root\n"},
			entry{path: "LICENSES/Apache.txt", text: "not at the root\n"},
			entry{path: "LICENSE-texts/", dir: true},
			entry{path: "LICENSE-texts/MIT.txt", text: "not at the root\n"},
			entry{path: "README.md", text: "no licence\n"}),
		"beta 0.2.0": archive(t, "beta", "0.2.0",
			described(manifest("beta", "0.2.0", "Unlicense OR MIT", "legal/TERMS.txt")),
			entry{path: "license-mit", text: mit},
			entry{path: "UNLICENSE", text: "This is free and unencumbered software."},
			entry{path: "COPYING", text: "Dual-licensed.\n"},
			entry{path: "legal/TERMS.txt", text: "The terms.\n"},
			entry{path: "NOTICE", text: "Apache License \n"}),
		"gamma 0.1.0": archive(t, "gamma", "0.1.0", manifest("gamma", "0.1.0", "MIT OR Apache-2.0", "")),
	}
	const head = "The crates' licence texts.\n\nThe crates:\n\n"
	dir, path := made(t, head, nil, crates, "beta 0.2.0", "alpha 1.0.0", "gamma 0.1.0")
	have, _ := os.ReadFile(path)
	list := strings.TrimPrefix(string(have[:bytes.Index(have, []byte("\n\n==> old"))+1]), head)
	var out strings.Builder
	if err := run(path, false, fromDir(dir), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "wrote "+path+" (3 crates,") {
		t.Errorf("run says %q", out.String())
	}
	got, _ := os.ReadFile(path)
	want := head + list +
		"\n==> beta 0.2.0 (Unlicense OR MIT) <==\n" +
		"--- COPYING\nDual-licensed.\n" +
		"--- NOTICE\nApache License \n" +
		"--- UNLICENSE\nThis is free and unencumbered software.\n" +
		"--- license-mit\n" + mit +
		"--- legal/TERMS.txt\nThe terms.\n" +
		"\n==> alpha 1.0.0 (MIT OR Apache-2.0) <==\n" +
		"--- LICENSE-APACHE\nApache License\n" +
		"--- LICENSE-MIT: the same text as beta 0.2.0 license-mit above\n" +
		"\n==> gamma 0.1.0 (MIT OR Apache-2.0) <==\n" +
		"No licence file in its archive; declared MIT OR Apache-2.0\n"
	if string(got) != want {
		t.Errorf("the file:\n%s\nwant:\n%s", got, want)
	}
	for _, check := range []bool{false, true} {
		out.Reset()
		if err := run(path, check, fromDir(dir), &out); err != nil || !strings.Contains(out.String(), "is current (3 crates)") {
			t.Errorf("run again, check %v: %v, %q", check, err, out.String())
		}
	}
}

// TestArchivesRefused checks that an archive the list does not vouch for
// stops the program, naming the crate and why, and leaves the file as it
// was: one whose SHA-256 is not the list's, one whose Cargo.toml names
// another version or its licence twice, a licence file that is a link
// or whose name holds a newline or a space, an entry outside the
// crate's directory, a text that is not UTF-8, that holds a control
// character or a line the file writes of its own; and -check on a file
// that is not current fails without writing.
func TestArchivesRefused(t *testing.T) {
	good := archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE", text: "MIT\n"})
	for _, c := range []struct {
		name    string
		archive []byte
		sum     string
		want    string
	}{
		{"its SHA-256 differs", good, strings.Repeat("0", 64), "delta 1.2.3: the archive's SHA-256 is"},
		{"another version", archive(t, "delta", "1.2.3", manifest("delta", "1.2.4", "MIT", ""), entry{path: "LICENSE", text: "MIT\n"}), "", "its Cargo.toml names delta 1.2.4"},
		{"a link", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE", link: "../../LICENSE"}), "", "delta-1.2.3/LICENSE is no regular file"},
		{"outside its directory", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "/elsewhere/LICENSE", text: "MIT\n"}), "", "elsewhere/LICENSE, outside delta-1.2.3/"},
		{"not UTF-8", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE", text: "\xff\n"}), "", "LICENSE is not UTF-8"},
		{"a line of the file's own", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE", text: "MIT\n==> other 1.0.0 (MIT) <==\n"}), "", "a line of LICENSE reads as one of the file's own"},
		{"no Cargo.toml", archive(t, "delta", "1.2.3", entry{path: "LICENSE", text: "MIT\n"}), "", "no delta-1.2.3/Cargo.toml in the archive"},
		{"its licence twice", archive(t, "delta", "1.2.3", entry{path: "Cargo.toml", text: manifest("delta", "1.2.3", "MIT", "").text + "[package]\nlicense = \"WTFPL\"\n"}, entry{path: "LICENSE", text: "MIT\n"}), "", "license is given twice in [package]"},
		{"a newline in a name", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE\n==> evil 9.9.9 (MIT) <==", text: "MIT\n"}), "", `the licence file "LICENSE\n==> evil 9.9.9 (MIT) <==" has a character in its name`},
		{"a space in a name", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE MIT", text: "MIT\n"}), "", `the licence file "LICENSE MIT" has a character in its name`},
		{"a control character", archive(t, "delta", "1.2.3", manifest("delta", "1.2.3", "MIT", ""), entry{path: "LICENSE", text: "MIT \x1b[31mred\n"}), "", "LICENSE holds the control character U+001B"},
	} {
		t.Run(c.name, func(t *testing.T) {
			sums := map[string]string{}
			if c.sum != "" {
				sums["delta 1.2.3"] = c.sum
			}
			dir, path := made(t, "Head.\n\n", sums, map[string][]byte{"delta 1.2.3": c.archive}, "delta 1.2.3")
			before, _ := os.ReadFile(path)
			err := run(path, false, fromDir(dir), io.Discard)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want one with %q", err, c.want)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
				t.Error("the file was changed")
			}
		})
	}
	dir, path := made(t, "Head.\n\n", nil, map[string][]byte{"delta 1.2.3": good}, "delta 1.2.3")
	before, _ := os.ReadFile(path)
	if err := run(path, true, fromDir(dir), io.Discard); err == nil || !strings.Contains(err.Error(), "is not what the archives give") {
		t.Errorf("-check on a file that is not current: %v", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Error("-check changed the file")
	}
}

// TestListRead checks the head's list: a line of name, version and
// SHA-256 a crate, every other line prose; a crate listed twice, a head
// that lists none, a CR in the head and a line that holds a SHA-256 but
// is no line of the list are errors.
func TestListRead(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	const commit = "254b59607d4417e9dffbc307138ae5c86280fe4c"
	head, list, err := split([]byte("Prose with 1.0.0 in it.\n\nserde_core 1.0.228 " + sum + "\nnot a line, at the commit " + commit + "\n\n\n==> serde_core 1.0.228 (MIT) <==\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(head, commit+"\n") || !slices.Equal(list, []crate{{"serde_core", "1.0.228", sum}}) {
		t.Errorf("head %q, list %v", head, list)
	}
	if _, _, err := split([]byte("a 1.0.0 " + sum + "\na 1.0.0 " + sum + "\n")); err == nil || !strings.Contains(err.Error(), "names a 1.0.0 twice") {
		t.Errorf("a crate twice: %v", err)
	}
	if _, _, err := split([]byte("Prose alone.\n")); err == nil || !strings.Contains(err.Error(), "lists no crate") {
		t.Errorf("no crate: %v", err)
	}
	if _, _, err := split([]byte("a 1.0.0 " + sum + "\r\nb 1.0.0 " + sum + "\n")); err == nil || !strings.Contains(err.Error(), "holds a CR") {
		t.Errorf("a CR: %v", err)
	}
	if _, _, err := split([]byte("a 1.0.0 " + sum + "\nb 1.0.0 " + sum + ".\n")); err == nil || !strings.Contains(err.Error(), "is no line") {
		t.Errorf("a stray character: %v", err)
	}
}
