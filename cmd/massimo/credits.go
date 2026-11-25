// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"go.metaworx.ch/massimo/internal/credits"
)

// runCredits prints the credits and licences of everything the binary
// includes, from the list it embeds, the one System info shows
// (internal/credits): it needs no store, no settings and no server.
func runCredits(args []string) error { return creditsCommand(args, os.Stdout) }

// creditsCommand is massimo credits writing to w: without a name the
// list, an entry a line; with one, that entry's licence, the text of
// each of its files.
func creditsCommand(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("credits", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: massimo credits [<name>]

  without a name  everything this binary includes, a line each: its name, version,
                  licence and where it comes from (System info's Credits and licences)
  <name>          that entry's licence, the text of each of its files; the name as
                  the list prints it, without regard to case
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return errors.New("credits: one name at most")
	}
	l, err := credits.Load()
	if err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return printCredits(w, l)
	}
	e, ok := l.Find(fs.Arg(0))
	if !ok {
		return fmt.Errorf("credits: %q is not in the list: massimo credits lists the names", fs.Arg(0))
	}
	if e.Includes != "" {
		fmt.Fprintf(w, "%s\n\n", includesLine(e))
	}
	for i, f := range e.Files {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "==> %s: %s (%s) <==\n%s", e.Name, f.Name, fileLicence(f), f.Text)
		if !strings.HasSuffix(f.Text, "\n") {
			fmt.Fprintln(w)
		}
	}
	return nil
}

// printCredits writes the list, a line an entry: its name, version,
// licence and where it comes from; then, for each entry that includes
// what it has no licence texts for, a line that says so.
func printCredits(w io.Writer, l credits.List) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVERSION\tLICENCE\tFROM")
	for _, e := range l.Entries {
		version := e.Version
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name, version, entryLicence(e), creditOrigin(e))
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("credits: %w", err)
	}
	for _, e := range l.Entries {
		if e.Includes != "" {
			fmt.Fprintf(w, "\n%s\n", includesLine(e))
		}
	}
	fmt.Fprintln(os.Stderr, "massimo credits <name> prints an entry's licence")
	return nil
}

// seeText stands for a licence whose text is known by no name.
const seeText = "see the text"

// entryLicence says an entry's licence, as System info does: its SPDX
// identifier, or that its text says it; "and others" where its files
// are not all under it.
func entryLicence(e credits.Entry) string {
	id := e.Licence
	if id == "" {
		id = seeText
	}
	if e.Mixed {
		id += " and others"
	}
	return id
}

// fileLicence says what a licence file is: its licence's identifier, a
// notice, or that its text says it.
func fileLicence(f credits.File) string {
	switch {
	case f.Notice:
		return "notice"
	case f.Licence == "":
		return seeText
	}
	return f.Licence
}

// includesLine says what else an entry includes, whose own licence
// texts the list does not hold.
func includesLine(e credits.Entry) string {
	return e.Name + " includes " + e.Includes + "; their own licence texts are not reproduced here."
}

// creditOrigin says where an entry comes from, as System info does.
func creditOrigin(e credits.Entry) string {
	switch e.Kind {
	case credits.KindSelf:
		return "this program"
	case credits.KindGo:
		return "Go's runtime and standard library"
	case credits.KindModule:
		if e.Path != "" {
			return "Go module, its copy in " + e.Path
		}
		return "Go module"
	}
	return e.Path
}
