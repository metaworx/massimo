// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command uibundle builds the web UI's parts: internal/server/ui/src/index.ts
// and what it imports, TypeScript and TSX, bundled by esbuild into
// internal/server/ui/parts.js, one classic script that the page loads after
// its plain scripts and the server embeds (docs/ui.md, "How the page is
// built"). Preact and its signals come from third_party/preact/, never from
// a node_modules; an import of anything else is an error.
//
// It runs from anywhere inside the checkout, the repository's top being
// the nearest directory above that holds the sources, as make ui runs it:
//
//	go -C tools/uibundle run .
//
// The same sources give the same bytes. On an error it prints esbuild's
// messages and exits 1, and leaves an existing bundle as it was.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Where the sources, the vendored library and the bundle are, from the
// repository's top.
const (
	entry  = "internal/server/ui/src/index.ts"
	out    = "internal/server/ui/parts.js"
	vendor = "third_party/preact"
)

// banner opens the bundle: the licence of Massimo's own code, and where
// the file comes from.
const banner = `// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors
// The web UI's parts, bundled by make ui (tools/uibundle) from
// internal/server/ui/src/; built, not committed, not edited by hand.`

// vendored are the modules the parts may import, each with its file
// under third_party/preact/, and the package it is of with that
// package's LICENSE there (third_party/preact/README.md).
var vendored = map[string]struct{ file, pkg, license string }{
	"preact":               {"preact/dist/preact.mjs", "preact", "preact/LICENSE"},
	"preact/hooks":         {"preact/hooks/dist/hooks.mjs", "preact", "preact/LICENSE"},
	"preact/jsx-runtime":   {"preact/jsx-runtime/dist/jsxRuntime.mjs", "preact", "preact/LICENSE"},
	"@preact/signals":      {"signals/dist/signals.mjs", "@preact/signals", "signals/LICENSE"},
	"@preact/signals-core": {"signals-core/dist/signals-core.mjs", "@preact/signals-core", "signals-core/LICENSE"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "uibundle:", err)
		os.Exit(1)
	}
}

// run finds the repository's top, bundles and writes the bundle.
func run() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := findRoot(wd)
	if err != nil {
		return err
	}
	b, err := bundle(root)
	if err != nil {
		return err
	}
	dst := filepath.Join(root, filepath.FromSlash(out))
	if old, err := os.ReadFile(dst); err == nil && bytes.Equal(old, b) {
		return nil
	}
	// A whole file or none: written beside the bundle, then moved over it.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".parts-*.js")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// findRoot is the nearest directory at or above dir that holds the entry
// point.
func findRoot(dir string) (string, error) {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, filepath.FromSlash(entry))); err == nil {
			return d, nil
		}
		up := filepath.Dir(d)
		if up == d {
			return "", fmt.Errorf("no %s at or above %s", entry, dir)
		}
		d = up
	}
}

// bundle is the bundle's bytes, made from the sources under root.
func bundle(root string) ([]byte, error) {
	res := api.Build(api.BuildOptions{
		AbsWorkingDir: root,
		EntryPoints:   []string{entry},
		Outfile:       out,
		Bundle:        true,
		Write:         false,
		Format:        api.FormatIIFE,
		Platform:      api.PlatformBrowser,
		Target:        api.ES2022,
		// The ES2022 of every browser the page supports; nothing is lowered
		// further.
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LegalComments:     api.LegalCommentsEndOfFile,
		Charset:           api.CharsetUTF8,
		JSX:               api.JSXAutomatic,
		JSXImportSource:   "preact",
		// The type check's tsconfig.json (src/) maps the imports to the
		// vendored types; the bundle takes none of it.
		TsconfigRaw: `{"compilerOptions":{}}`,
		Banner:      map[string]string{"js": banner},
		Sourcemap:   api.SourceMapNone,
		LogLevel:    api.LogLevelSilent,
		Plugins:     []api.Plugin{vendoredOnly(root)},
	})
	if len(res.Errors) > 0 {
		return nil, errors.New(messages(res.Errors, api.ErrorMessage))
	}
	if len(res.Warnings) > 0 {
		// A warning is a mistake in the sources too: a part has none.
		return nil, errors.New(messages(res.Warnings, api.WarningMessage))
	}
	if len(res.OutputFiles) != 1 {
		return nil, fmt.Errorf("esbuild wrote %d files, want 1", len(res.OutputFiles))
	}
	return res.OutputFiles[0].Contents, nil
}

// messages are esbuild's messages as it prints them, the file and line
// of each with the line itself.
func messages(msgs []api.Message, kind api.MessageKind) string {
	return strings.TrimRight(strings.Join(api.FormatMessages(msgs, api.FormatMessagesOptions{Kind: kind}), ""), "\n")
}

// bare is an import that is no path: a package's name.
var bare = regexp.MustCompile(`^[^./]`)

// vendoredOnly resolves the vendored modules to their files under
// third_party/preact/ and refuses any other package. Each vendored file
// is loaded with a legal comment naming its package's licence, which
// esbuild keeps at the bundle's end; the files themselves carry none.
func vendoredOnly(root string) api.Plugin {
	return api.Plugin{
		Name: "vendored-only",
		Setup: func(b api.PluginBuild) {
			b.OnResolve(api.OnResolveOptions{Filter: `.*`}, func(a api.OnResolveArgs) (api.OnResolveResult, error) {
				if a.Kind == api.ResolveEntryPoint || !bare.MatchString(a.Path) {
					return api.OnResolveResult{}, nil
				}
				m, ok := vendored[a.Path]
				if !ok {
					return api.OnResolveResult{}, fmt.Errorf("%s is not vendored: the parts import preact, preact/hooks, preact/jsx-runtime, @preact/signals and @preact/signals-core alone (%s/README.md)", a.Path, vendor)
				}
				return api.OnResolveResult{Path: filepath.Join(root, filepath.FromSlash(vendor+"/"+m.file)), Namespace: "file"}, nil
			})
			files := map[string][2]string{} // by absolute path: the package and its licence
			for _, m := range vendored {
				files[filepath.Join(root, filepath.FromSlash(vendor+"/"+m.file))] = [2]string{m.pkg, m.license}
			}
			b.OnLoad(api.OnLoadOptions{Filter: `\.mjs$`}, func(a api.OnLoadArgs) (api.OnLoadResult, error) {
				of, ok := files[a.Path]
				if !ok {
					return api.OnLoadResult{}, nil
				}
				code, err := os.ReadFile(a.Path)
				if err != nil {
					return api.OnLoadResult{}, err
				}
				note, err := legalComment(root, of[0], of[1])
				if err != nil {
					return api.OnLoadResult{}, err
				}
				s := note + "\n" + string(code)
				return api.OnLoadResult{Contents: &s, Loader: api.LoaderJS, ResolveDir: filepath.Dir(a.Path)}, nil
			})
		},
	}
}

// legalComment is the comment that credits a vendored package: its name,
// the title and the copyright line of its LICENSE, and where the whole
// text is.
func legalComment(root, pkg, license string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(vendor+"/"+license)))
	if err != nil {
		return "", err
	}
	var title, copyright string
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case title == "" && line != "":
			title = line
		case copyright == "" && strings.HasPrefix(line, "Copyright"):
			copyright = line
		}
	}
	if copyright == "" {
		return "", fmt.Errorf("%s/%s: no copyright line", vendor, license)
	}
	return fmt.Sprintf("/*! %s: %s, %s, %s/%s */", pkg, title, copyright, vendor, license), nil
}
