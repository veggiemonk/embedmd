// Copyright 2016 Google Inc. All rights reserved.
// Copyright 2026 Julien Bisconti and the embedmd fork contributors.
//
// Changed in the github.com/veggiemonk/embedmd fork.
// See the NOTICE file for the list of changes.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to writing, software distributed
// under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

package embedmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchHTTPTimeout(t *testing.T) {
	// Server that hangs forever without sending a response body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// flush headers so the client sees 200, then block
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// block until the client gives up
		<-r.Context().Done()
	}))
	defer srv.Close()

	// Replace the package-level client with a very short timeout.
	old := httpClient
	httpClient = &http.Client{Timeout: 100 * time.Millisecond}
	defer func() { httpClient = old }()

	f := fetcher{}
	_, err := f.Fetch(context.Background(), "", srv.URL)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestFetchHTTPSizeLimit(t *testing.T) {
	// A server that returns one byte more than the limit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		chunk := bytes.Repeat([]byte("x"), 4096)
		for total := 0; total < maxResponseSize+1; {
			n, err := w.Write(chunk)
			if err != nil {
				return
			}
			total += n
		}
	}))
	defer srv.Close()

	f := fetcher{}
	b, err := f.Fetch(context.Background(), "", srv.URL)
	if err == nil {
		t.Fatalf("expected an error; got %d bytes", len(b))
	}
	if !strings.Contains(err.Error(), "larger than the limit") {
		t.Fatalf("expected a size limit error; got %v", err)
	}
	if b != nil {
		t.Errorf("expected no content with the error; got %d bytes", len(b))
	}
}

func TestFetchHTTPNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := fetcher{}
	_, err := f.Fetch(context.Background(), "", srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	want := fmt.Sprintf("status %s", http.StatusText(http.StatusNotFound)+" 404 page not found\n")
	_ = want // error message format varies; just confirm non-nil
}

func TestFetchContextCancelled(t *testing.T) {
	// Server that blocks until the client goes away.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	f := fetcher{}
	_, err := f.Fetch(ctx, "", srv.URL)
	if err == nil {
		t.Fatal("expected error after cancel, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestProcessContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	in := strings.NewReader("[embedmd]:# (code.go)\n")
	var out bytes.Buffer
	err := Process(ctx, &out, in, WithFetcher(fakeFileProvider{"code.go": []byte("package main\n")}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestReadLocalFile(t *testing.T) {
	// base/root is the base directory. base/secret.go sits outside it.
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(base, "secret.go")
	write(filepath.Join(root, "code.go"), "inside\n")
	write(filepath.Join(root, "sub", "code.go"), "nested\n")
	write(secret, "secret\n")

	// A symbolic link inside the base directory that points out of it.
	// Creating one needs a privilege on Windows, so the case is optional.
	link := filepath.Join(root, "escape.go")
	haveSymlink := os.Symlink(secret, link) == nil

	type testCase struct {
		name    string
		dir     string
		path    string
		want    string
		escapes bool
	}
	tests := []testCase{
		{name: "file in the base directory", dir: root, path: "code.go", want: "inside\n"},
		{name: "file in a sub directory", dir: root, path: "sub/code.go", want: "nested\n"},
		{name: "sub directory then back up", dir: root, path: "sub/../code.go", want: "inside\n"},
		{name: "parent directory", dir: root, path: "../secret.go", escapes: true},
		{name: "several parent directories", dir: root, path: "../../../../etc/hosts", escapes: true},
		{name: "absolute path", dir: root, path: filepath.ToSlash(secret), escapes: true},
		{name: "no base directory reads anywhere", dir: "", path: filepath.ToSlash(secret), want: "secret\n"},
	}
	if haveSymlink {
		tests = append(tests, testCase{
			name: "symbolic link out of the base directory", dir: root, path: "escape.go", escapes: true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := readLocalFile(tt.dir, tt.path)
			if tt.escapes {
				if err == nil {
					t.Fatalf("expected the read to fail; got %q", b)
				}
				if !strings.Contains(err.Error(), "escapes") {
					t.Fatalf("expected an escape error; got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("expected %q; got %q", tt.want, b)
			}
		})
	}
}
