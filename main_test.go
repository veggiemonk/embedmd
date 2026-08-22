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
// under the License is distributed on a "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEmbedNoPaths(t *testing.T) {
	tests := []struct {
		name string
		err  string
		d, w bool
	}{
		{
			name: "no files provided",
			err:  "error: no markdown files provided",
		},
		{
			name: "can't diff and rewrite",
			w:    true, d: true,
			err: "error: cannot use -w and -d simultaneously",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := embed(context.Background(), nil, tt.w, tt.d)
			if err == nil || err.Error() != tt.err {
				t.Fatalf("expected error %q; got %v", tt.err, err)
			}
		})
	}
}

// newDoc writes docs.md with the given content in a new directory, next to a
// hello.go that a directive can embed, and returns the path of docs.md.
func newDoc(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "docs.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEmbedRewrite(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "a directive gets its block",
			in:   "[embedmd]:# (hello.go)\n",
			out:  "[embedmd]:# (hello.go)\n```go\nhi\n```\n",
		},
		{
			name: "an up to date block stays as it is",
			in:   "[embedmd]:# (hello.go)\n```go\nhi\n```\n",
			out:  "[embedmd]:# (hello.go)\n```go\nhi\n```\n",
		},
		{
			name: "no line terminator is added",
			in:   "one\ntwo\nthree",
			out:  "one\ntwo\nthree",
		},
		{
			name: "CRLF stays CRLF",
			in:   "one\r\ntwo\r\n",
			out:  "one\r\ntwo\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := newDoc(t, tt.in)
			if _, err := embed(context.Background(), []string{path}, true, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.out {
				t.Errorf("expected file\n%q; got\n%q", tt.out, got)
			}
		})
	}
}

func TestEmbedRewriteKeepsFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes work differently on Windows")
	}
	path := newDoc(t, "[embedmd]:# (hello.go)\n")
	if _, err := embed(context.Background(), []string{path}, true, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("expected mode 0644; got %04o", got)
	}
}

func TestEmbedRewriteLeavesNoTemporaryFile(t *testing.T) {
	path := newDoc(t, "[embedmd]:# (hello.go)\n")
	if _, err := embed(context.Background(), []string{path}, true, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 2 {
		t.Errorf("expected docs.md and hello.go only; got %v", names)
	}
}

func TestEmbedDiff(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		out       string
		foundDiff bool
	}{
		{
			name:      "a missing block shows as added",
			in:        "[embedmd]:# (hello.go)\n",
			out:       "@@ -1,2 +1,5 @@\n [embedmd]:# (hello.go)\n+```go\n+hi\n+```\n \n",
			foundDiff: true,
		},
		{
			name: "an up to date file shows nothing",
			in:   "[embedmd]:# (hello.go)\n```go\nhi\n```\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := newDoc(t, tt.in)
			var out bytes.Buffer
			old := stdout
			stdout = &out
			t.Cleanup(func() { stdout = old })

			foundDiff, err := embed(context.Background(), []string{path}, false, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if foundDiff != tt.foundDiff {
				t.Errorf("expected foundDiff %v; got %v", tt.foundDiff, foundDiff)
			}
			if got := out.String(); got != tt.out {
				t.Errorf("expected diff\n%q; got\n%q", tt.out, got)
			}
		})
	}

	// The file must not change under -d.
	path := newDoc(t, "[embedmd]:# (hello.go)\n")
	var out bytes.Buffer
	old := stdout
	stdout = &out
	t.Cleanup(func() { stdout = old })
	if _, err := embed(context.Background(), []string{path}, false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "[embedmd]:# (hello.go)\n" {
		t.Errorf("the file changed under -d: %q", got)
	}
}

func TestEmbedNotMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.go")
	if err := os.WriteFile(path, []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := embed(context.Background(), []string{path}, false, false)
	if err == nil || !strings.Contains(err.Error(), "not a markdown file") {
		t.Fatalf("expected a markdown error; got %v", err)
	}
}
