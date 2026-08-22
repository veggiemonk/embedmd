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
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
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
			a := app{stdout: io.Discard, stderr: io.Discard}
			_, err := a.embed(context.Background(), nil, tt.w, tt.d)
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
			a := app{stdout: io.Discard, stderr: io.Discard}
			if _, err := a.embed(context.Background(), []string{path}, true, false); err != nil {
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
	a := app{stdout: io.Discard, stderr: io.Discard}
	if _, err := a.embed(context.Background(), []string{path}, true, false); err != nil {
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
	a := app{stdout: io.Discard, stderr: io.Discard}
	if _, err := a.embed(context.Background(), []string{path}, true, false); err != nil {
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
			out:       "@@ -1 +1,4 @@\n [embedmd]:# (hello.go)\n+```go\n+hi\n+```\n",
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
			a := app{stdout: &out, stderr: io.Discard}

			foundDiff, err := a.embed(context.Background(), []string{path}, false, true)
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
	a := app{stdout: &out, stderr: io.Discard}
	if _, err := a.embed(context.Background(), []string{path}, false, true); err != nil {
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
	a := app{stdout: io.Discard, stderr: io.Discard}
	_, err := a.embed(context.Background(), []string{path}, false, false)
	if err == nil || !strings.Contains(err.Error(), "not a markdown file") {
		t.Fatalf("expected a markdown error; got %v", err)
	}
}

func TestEmbedReportsEveryFailure(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, name := range []string{"one.md", "two.md"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("[embedmd]:# (missing.go)\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	a := app{stdout: io.Discard, stderr: io.Discard}
	_, err := a.embed(context.Background(), paths, false, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, path := range paths {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("expected %s in the report; got %v", path, err)
		}
	}
}

func TestRunExitStatus(t *testing.T) {
	upToDate := newDoc(t, "[embedmd]:# (hello.go)\n```go\nhi\n```\n")
	stale := newDoc(t, "[embedmd]:# (hello.go)\n")
	missing := newDoc(t, "[embedmd]:# (nowhere.go)\n")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "nothing to report", args: []string{"-d", upToDate}, want: exitOK},
		{name: "a difference", args: []string{"-d", stale}, want: exitDiffFound},
		{name: "a failure", args: []string{"-d", missing}, want: exitError},
		{name: "the version", args: []string{"-v"}, want: exitOK},
		{name: "-w and -d together", args: []string{"-w", "-d", upToDate}, want: exitError},
		{name: "no file", args: nil, want: exitError},
		{name: "an unknown flag", args: []string{"-nope"}, want: exitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(context.Background(), tt.args, io.Discard, io.Discard)
			if got != tt.want {
				t.Errorf("expected exit status %d; got %d", tt.want, got)
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	buildInfo := func(mainVersion, revision, modified string) *debug.BuildInfo {
		info := &debug.BuildInfo{}
		info.Main.Version = mainVersion
		if revision != "" {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.revision", Value: revision})
		}
		if modified != "" {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.modified", Value: modified})
		}
		return info
	}

	tests := []struct {
		name  string
		stamp string
		info  *debug.BuildInfo
		want  string
	}{
		{
			name:  "a stamped release wins",
			stamp: "v1.2.3",
			info:  buildInfo("(devel)", "0123456789abcdef", "false"),
			want:  "v1.2.3",
		},
		{
			name: "go install records the version",
			info: buildInfo("v1.2.3", "", ""),
			want: "v1.2.3",
		},
		{
			name: "a build from a clean tree shows the commit",
			info: buildInfo("(devel)", "0123456789abcdef", "false"),
			want: "0123456789ab",
		},
		{
			name: "a build from a changed tree says so",
			info: buildInfo("(devel)", "0123456789abcdef", "true"),
			want: "0123456789ab+dirty",
		},
		{
			name: "a pseudo-version is already marked",
			info: buildInfo("v0.0.0-20260101000000-0123456789ab+dirty", "0123456789abcdef", "true"),
			want: "v0.0.0-20260101000000-0123456789ab+dirty",
		},
		{
			name: "nothing to go on",
			info: buildInfo("", "", ""),
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatVersion(tt.stamp, tt.info); got != tt.want {
				t.Errorf("expected %q; got %q", tt.want, got)
			}
		})
	}
}
