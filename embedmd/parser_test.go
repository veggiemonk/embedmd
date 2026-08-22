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

package embedmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/veggiemonk/embedmd/internal/testutil"
)

func TestParser(t *testing.T) {
	tc := []struct {
		name string
		in   string
		out  string
		run  commandRunner
		err  string
	}{
		{
			name: "empty file",
			in:   "",
			out:  "",
		},
		{
			name: "just text",
			in:   "one\ntwo\nthree\n",
			out:  "one\ntwo\nthree\n",
		},
		{
			name: "a command",
			in:   "one\n[embedmd]:# (code.go)",
			out:  "one\n[embedmd]:# (code.go)\nOK\n",
			run: func(w io.Writer, cmd *command, eol string) error {
				if cmd.path != "code.go" {
					return fmt.Errorf("bad command")
				}
				fmt.Fprint(w, "OK\n")
				return nil
			},
		},
		{
			name: "a command then some text",
			in:   "one\n[embedmd]:# (code.go)\nYay\n",
			out:  "one\n[embedmd]:# (code.go)\nOK\nYay\n",
			run: func(w io.Writer, cmd *command, eol string) error {
				if cmd.path != "code.go" {
					return fmt.Errorf("bad command")
				}
				fmt.Fprint(w, "OK\n")
				return nil
			},
		},
		{
			name: "a bad command",
			in:   "one\n[embedmd]:# (code\n",
			err:  "2: argument list should be in parenthesis",
		},
		{
			name: "an ignored command",
			in:   "one\n```\n[embedmd]:# (code.go)\n```\n",
			out:  "one\n```\n[embedmd]:# (code.go)\n```\n",
		},
		{
			name: "unbalanced code section",
			in:   "one\n```\nsome code\n",
			err:  "3: unbalanced code section",
		},
		{
			name: "two contiguous code sections",
			in:   "\n```go\nhello\n```\n```go\nbye\n```\n",
			out:  "\n```go\nhello\n```\n```go\nbye\n```\n",
		},
		{
			name: "two non contiguous code sections",
			in:   "```go\nhello\n```\n\n```go\nbye\n```\n",
			out:  "```go\nhello\n```\n\n```go\nbye\n```\n",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := process(&out, strings.NewReader(tt.in), tt.run)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}
			if got := out.String(); got != tt.out {
				t.Errorf("case [%s] expected %q; got %q", tt.name, tt.out, got)
			}
		})
	}
}

func TestProcessKeepsLineEndings(t *testing.T) {
	run := func(w io.Writer, cmd *command, eol string) error {
		_, err := io.WriteString(w, "```"+eol+"code"+eol+"```"+eol)
		return err
	}

	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "CRLF stays CRLF",
			in:   "one\r\ntwo\r\n```\r\nkept\r\n```\r\n",
			out:  "one\r\ntwo\r\n```\r\nkept\r\n```\r\n",
		},
		{
			name: "no terminator on the last line stays missing",
			in:   "one\ntwo",
			out:  "one\ntwo",
		},
		{
			name: "mixed terminators each stay as they are",
			in:   "one\r\ntwo\nthree\r\n",
			out:  "one\r\ntwo\nthree\r\n",
		},
		{
			name: "a generated block follows the terminator of its directive",
			in:   "[embedmd]:# (code.go)\r\ntext\r\n",
			out:  "[embedmd]:# (code.go)\r\n```\r\ncode\r\n```\r\ntext\r\n",
		},
		{
			name: "a directive on the last line gets a terminator",
			in:   "[embedmd]:# (code.go)",
			out:  "[embedmd]:# (code.go)\n```\ncode\n```\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := process(&out, strings.NewReader(tt.in), run); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := out.String(); got != tt.out {
				t.Errorf("expected %q; got %q", tt.out, got)
			}
		})
	}
}

func TestProcessLongLine(t *testing.T) {
	// bufio.Scanner refuses a line longer than 64 KiB, and reported it
	// against line 0, which does not exist.
	long := strings.Repeat("x", 70000)
	in := "first\n" + long + "\nlast\n"

	var out bytes.Buffer
	if err := process(&out, strings.NewReader(in), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != in {
		t.Errorf("expected %d bytes back; got %d", len(in), len(got))
	}
}

func TestProcessWriteError(t *testing.T) {
	want := errors.New("disk is full")
	err := process(failingWriter{want}, strings.NewReader("one\ntwo\n"), nil)
	if !errors.Is(err, want) {
		t.Fatalf("expected %v; got %v", want, err)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }
