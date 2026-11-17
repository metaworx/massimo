// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package safefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// daclOf is the DACL of the file at path in SDDL.
func daclOf(t *testing.T, path string) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	return sd.String()
}

// The replaced file's DACL, protected from the directory's inheritance,
// is the new file's.
func TestDACLKept(t *testing.T) {
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, []byte("old"), 0o600)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	// SYSTEM, the administrators and this user in full, the users reading.
	sddl := "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + user.User.Sid.String() + ")(A;;FR;;;BU)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, _ := sd.DACL()
	if err := windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	before := daclOf(t, p)
	if err := Write(p, []byte("new"), Options{}); err != nil {
		t.Fatal(err)
	}
	if after := daclOf(t, p); after != before || !strings.HasPrefix(after, "D:P") {
		t.Fatalf("DACL %s, want %s", after, before)
	}
	if got := readString(t, p); got != "new" {
		t.Fatalf("content %q", got)
	}
}

// A rename over a file a reader holds is retried until the reader lets
// go.
func TestRenameRetriesAHeldFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "status.json")
	os.WriteFile(p, []byte("old"), 0o600)
	held, err := os.Open(p) // without FILE_SHARE_DELETE, as Go opens files
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		held.Close()
	}()
	if err := Write(p, []byte("new"), Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, p); got != "new" {
		t.Fatalf("content %q", got)
	}
}

// A rename of a file another process holds - a virus scanner reading the
// .exe an update just installed - waits until it lets go, and so does a
// removal; os.Rename itself is refused meanwhile.
func TestRenameAndRemoveWaitForAHeldFile(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "massimo.exe"), filepath.Join(dir, "massimo.exe.prev")
	if err := os.WriteFile(from, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	hold := func(p string) {
		f, err := os.Open(p) // without FILE_SHARE_DELETE, as Go opens files
		if err != nil {
			t.Fatal(err)
		}
		time.AfterFunc(300*time.Millisecond, func() { f.Close() })
	}
	hold(from)
	if err := os.Rename(from, to); err == nil {
		t.Fatal("os.Rename renamed a held file: the test holds nothing")
	}
	if err := Rename(from, to, 0); err != nil {
		t.Fatalf("Rename of a held file: %v", err)
	}
	if got := readString(t, to); got != "new" {
		t.Fatalf("content %q", got)
	}
	hold(to)
	if err := Remove(to, 0); err != nil {
		t.Fatalf("Remove of a held file: %v", err)
	}
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		t.Fatalf("the file is still there: %v", err)
	}
}
