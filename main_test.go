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
	"strings"
	"testing"

	"github.com/veggiemonk/embedmd/internal/testutil"
)

func TestEmbedNoPaths(t *testing.T) {
	tc := []struct {
		name string
		err  string
		d, w bool
	}{
		{name: "no files provided",
			err: "error: no markdown files provided",
		},
		{name: "can't diff and rewrite",
			w: true, d: true,
			err: "error: cannot use -w and -d simultaneously",
		},
	}

	for _, tt := range tc {
		_, err := embed(context.Background(), nil, tt.w, tt.d)
		testutil.EqErr(t, tt.name, err, tt.err)
	}
}

func TestEmbedFiles(t *testing.T) {
	// The markdown file is a fake, but the file it embeds is a real one, so
	// the fetcher does real work.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "docs.md")

	tc := []struct {
		name string
		in   string
		out  string
		err  string
		d, w bool
	}{
		{name: "rewriting a file",
			in:  "[embedmd]:# (hello.go)\n",
			w:   true,
			out: "[embedmd]:# (hello.go)\n```go\nhi\n```\n",
		},
		{name: "rewriting adds no line terminator of its own",
			in:  "one\ntwo\nthree",
			w:   true,
			out: "one\ntwo\nthree",
		},
		{name: "rewriting keeps CRLF",
			in:  "one\r\ntwo\r\n",
			w:   true,
			out: "one\r\ntwo\r\n",
		},
		{name: "diffing a file",
			in:  "[embedmd]:# (hello.go)\n",
			d:   true,
			out: "@@ -1,2 +1,5 @@\n [embedmd]:# (hello.go)\n+```go\n+hi\n+```\n \n",
		},
	}

	defer func(f func(string) (file, error)) { openFile = f }(openFile)

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeFile(tt.in)
			openFile = func(string) (file, error) { return f, nil }
			stdout = os.Stdout
			if tt.d {
				stdout = &f.buf
			}

			_, err := embed(context.Background(), []string{path}, tt.w, tt.d)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}
			if got := f.buf.String(); tt.out != got {
				t.Errorf("expected output \n%q; got\n%q", tt.out, got)
			}
		})
	}
}

type fakeFile struct {
	io.ReadCloser
	buf bytes.Buffer
}

func (f *fakeFile) WriteAt(b []byte, offset int64) (int, error) { return f.buf.Write(b) }
func (f *fakeFile) Truncate(int64) error                        { return nil }

func newFakeFile(s string) *fakeFile {
	return &fakeFile{ReadCloser: io.NopCloser(strings.NewReader(s))}
}
