// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
)

// Once the Cube of the plain MASSIMO_CUBE_ADDRESS reported its serial,
// the server writes its row and the setting that ties the plain lines to
// it; a stop between the two must leave a store the next start serves as
// one Cube. An SQLite trigger refuses one of the writes, which cuts the
// run short there. The setting refused used to leave the row alone, and
// the next start refused the row and the plain lines as several Cubes.
func TestServePlainSeedCutShort(t *testing.T) {
	for _, c := range []struct{ name, trigger string }{
		{"the setting refused", `CREATE TRIGGER cut BEFORE INSERT ON settings WHEN NEW.key = 'seed.plain_serial' BEGIN SELECT RAISE(ABORT, 'cut short'); END`},
		{"the row refused", `CREATE TRIGGER cut BEFORE INSERT ON cubes BEGIN SELECT RAISE(ABORT, 'cut short'); END`},
	} {
		t.Run(c.name, func(t *testing.T) {
			clearCubeEnv(t)
			cubeAddr := startFakeCube(t)
			envFile := filepath.Join(t.TempDir(), "x.env")
			os.WriteFile(envFile, []byte("MASSIMO_CUBE_ADDRESS="+cubeAddr+"\nMASSIMO_CUBE_PROXY="+freeAddr(t)+"\n"), 0o600)
			if err := env.LoadFile(envFile); err != nil {
				t.Fatal(err)
			}
			st := testStore(t, "")
			ctx := context.Background()
			other, err := sql.Open("sqlite", "file:"+filepath.ToSlash(st.File())+"?_pragma=busy_timeout(5000)")
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if _, err := other.Exec(c.trigger); err != nil {
				t.Fatal(err)
			}

			su, err := serveSetup(ctx, st, "", nil, "", quietLog())
			if err != nil {
				t.Fatal(err)
			}
			var log syncLog
			srv := server.New(server.Options{Cubes: su.cubes, Proxy: su.listeners, Store: st, FileCubes: su.fileCubes, Logger: slog.New(slog.NewTextHandler(&log, nil))})
			runCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				srv.Run(runCtx)
				close(done)
			}()
			for deadline := time.Now().Add(10 * time.Second); !strings.Contains(log.String(), "cut short"); time.Sleep(20 * time.Millisecond) {
				if time.Now().After(deadline) {
					cancel()
					<-done
					t.Fatalf("no write refused:\n%s", log.String())
				}
			}
			cancel()
			<-done
			if _, err := other.Exec(`DROP TRIGGER cut`); err != nil {
				t.Fatal(err)
			}

			su, err = serveSetup(ctx, st, "", nil, "", quietLog())
			if err != nil || len(su.cubes) != 1 {
				t.Fatalf("the next start: %d cubes, %v", len(su.cubes), err)
			}
		})
	}
}

// syncLog collects a log written from several goroutines.
type syncLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *syncLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *syncLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
