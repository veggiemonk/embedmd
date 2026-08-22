// Copyright 2016 Google Inc. All rights reserved.
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
	"testing"

	"github.com/campoy/embedmd/internal/testutil"
)

func TestFields(t *testing.T) {
	tc := []struct {
		name string
		in   string
		out  []string
		err  string
	}{
		{
			name: "simple args",
			in:   "code.go go /start/ /end/", out: []string{"code.go", "go", "/start/", "/end/"},
		},
		{
			name: "exclude regexp",
			in:   "code.go go !/start/ !/end/", out: []string{"code.go", "go", "!/start/", "!/end/"},
		},
		{
			name: "exclude regexp with spaces",
			in:   `code.go go !/start here/ !/end here/`, out: []string{"code.go", "go", "!/start here/", "!/end here/"},
		},
		{
			name: "substitution",
			in:   "code.go go /start/ /end/ s/ELLIPSIS/.../", out: []string{"code.go", "go", "/start/", "/end/", "s/ELLIPSIS/.../"},
		},
		{
			name: "substitution with spaces in old",
			in:   "code.go go s/_ = ELLIPSIS/.../", out: []string{"code.go", "go", "s/_ = ELLIPSIS/.../"},
		},
		{
			name: "options after regexps",
			in:   "code.go go !/start/ !/end/ dedent trim", out: []string{"code.go", "go", "!/start/", "!/end/", "dedent", "trim"},
		},
		{
			name: "unbalanced exclude regexp",
			in:   "code.go !/start", err: "unbalanced /",
		},
		{
			name: "unbalanced substitution first sep",
			in:   "code.go s/broken", err: "unbalanced / in substitution",
		},
		{
			name: "unbalanced substitution second sep",
			in:   "code.go s/old/broken", err: "unbalanced / in substitution",
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fields(tt.in)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}
			if len(got) != len(tt.out) {
				t.Fatalf("case [%s]: expected %d fields %v; got %d fields %v", tt.name, len(tt.out), tt.out, len(got), got)
			}
			for i := range got {
				if got[i] != tt.out[i] {
					t.Errorf("case [%s]: field %d: expected %q; got %q", tt.name, i, tt.out[i], got[i])
				}
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	tc := []struct {
		name string
		in   string
		cmd  command
		err  string
	}{
		{
			name: "start to end",
			in:   "(code.go /start/ /end/)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/")},
		},
		{
			name: "only start",
			in:   "(code.go     /start/)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/")},
		},
		{
			name: "empty list",
			in:   "()",
			err:  "missing file name",
		},
		{
			name: "file with no extension and no lang",
			in:   "(test)",
			err:  "language is required when file has no extension",
		},
		{
			name: "surrounding blanks",
			in:   "   \t  (code.go)  \t  ",
			cmd:  command{path: "code.go", lang: "go"},
		},
		{
			name: "no parenthesis",
			in:   "{code.go}",
			err:  "argument list should be in parenthesis",
		},
		{
			name: "only left parenthesis",
			in:   "(code.go",
			err:  "argument list should be in parenthesis",
		},
		{
			name: "regexp not closed",
			in:   "(code.go /start)",
			err:  "unbalanced /",
		},
		{
			name: "end regexp not closed",
			in:   "(code.go /start/ /end)",
			err:  "unbalanced /",
		},
		{
			name: "file name and language",
			in:   "(test.md markdown)",
			cmd:  command{path: "test.md", lang: "markdown"},
		},
		{
			name: "multi-line comments",
			in:   `(doc.go /\/\*/ /\*\//)`,
			cmd:  command{path: "doc.go", lang: "go", start: testutil.Ptr(`/\/\*/`), end: testutil.Ptr(`/\*\//`)},
		},
		{
			name: "using $ as end",
			in:   "(foo.go /start/ $)",
			cmd:  command{path: "foo.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("$")},
		},
		{
			name: "exclude start",
			in:   "(code.go !/start/ /end/)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"), excludeStart: true},
		},
		{
			name: "exclude end",
			in:   "(code.go /start/ !/end/)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"), excludeEnd: true},
		},
		{
			name: "exclude both",
			in:   "(code.go !/start/ !/end/)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"), excludeStart: true, excludeEnd: true},
		},
		{
			name: "exclude start with dollar end",
			in:   "(code.go !/start/ $)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("$"), excludeStart: true},
		},
		{
			name: "dedent option",
			in:   "(code.go /start/ /end/ dedent)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"), dedent: true},
		},
		{
			name: "trim option",
			in:   "(code.go /start/ /end/ trim)",
			cmd:  command{path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"), trim: true},
		},
		{
			name: "substitution option",
			in:   "(code.go /start/ /end/ s/ELLIPSIS/.../)",
			cmd: command{
				path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"),
				substitutions: []substitution{{old: "ELLIPSIS", new: "..."}},
			},
		},
		{
			name: "multiple substitutions",
			in:   "(code.go s/ELLIPSIS/.../ s/_ = ELLIPSIS/.../)",
			cmd: command{
				path: "code.go", lang: "go",
				substitutions: []substitution{{old: "ELLIPSIS", new: "..."}, {old: "_ = ELLIPSIS", new: "..."}},
			},
		},
		{
			name: "all options combined",
			in:   "(code.go !/start/ !/end/ dedent trim s/ELLIPSIS/.../)",
			cmd: command{
				path: "code.go", lang: "go", start: testutil.Ptr("/start/"), end: testutil.Ptr("/end/"),
				excludeStart: true, excludeEnd: true, dedent: true, trim: true,
				substitutions: []substitution{{old: "ELLIPSIS", new: "..."}},
			},
		},
		{
			name: "exclude on single regexp is error",
			in:   "(code.go !/only/)",
			err:  "exclude (!) cannot be used with a single regexp",
		},
		{
			name: "exclude on dollar is error",
			in:   "(code.go /start/ !$)",
			err:  "exclude (!) cannot be used with $",
		},
		{
			name: "unknown option",
			in:   "(code.go /start/ /end/ bogus)",
			err:  `unknown option "bogus"`,
		},
		{
			name: "invalid substitution",
			in:   "(code.go /start/ /end/ s/broken)",
			err:  "unbalanced / in substitution",
		},
		{
			name: "extra arguments",
			in:   "(foo.go /start/ $ extra)", err: "unknown option \"extra\"",
		},
		{
			name: "file name with directories",
			in:   "(foo/bar.go)",
			cmd:  command{path: "foo/bar.go", lang: "go"},
		},
		{
			name: "url",
			in:   "(http://golang.org/sample.go)",
			cmd:  command{path: "http://golang.org/sample.go", lang: "go"},
		},
		{
			name: "bad url",
			in:   "(http://golang:org:sample.go)",
			cmd:  command{path: "http://golang:org:sample.go", lang: "go"},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := parseCommand(tt.in)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}

			want, got := tt.cmd, *cmd
			if want.path != got.path {
				t.Errorf("case [%s]: expected file %q; got %q", tt.name, want.path, got.path)
			}
			if want.lang != got.lang {
				t.Errorf("case [%s]: expected language %q; got %q", tt.name, want.lang, got.lang)
			}
			if !testutil.EqPtr(want.start, got.start) {
				t.Errorf("case [%s]: expected start %v; got %v", tt.name, testutil.Str(want.start), testutil.Str(got.start))
			}
			if !testutil.EqPtr(want.end, got.end) {
				t.Errorf("case [%s]: expected end %v; got %v", tt.name, testutil.Str(want.end), testutil.Str(got.end))
			}
			if want.excludeStart != got.excludeStart {
				t.Errorf("case [%s]: expected excludeStart %v; got %v", tt.name, want.excludeStart, got.excludeStart)
			}
			if want.excludeEnd != got.excludeEnd {
				t.Errorf("case [%s]: expected excludeEnd %v; got %v", tt.name, want.excludeEnd, got.excludeEnd)
			}
			if want.dedent != got.dedent {
				t.Errorf("case [%s]: expected dedent %v; got %v", tt.name, want.dedent, got.dedent)
			}
			if want.trim != got.trim {
				t.Errorf("case [%s]: expected trim %v; got %v", tt.name, want.trim, got.trim)
			}
			if len(want.substitutions) != len(got.substitutions) {
				t.Errorf("case [%s]: expected %d substitutions; got %d", tt.name, len(want.substitutions), len(got.substitutions))
			} else {
				for i := range want.substitutions {
					if want.substitutions[i] != got.substitutions[i] {
						t.Errorf("case [%s]: substitution %d: expected %v; got %v", tt.name, i, want.substitutions[i], got.substitutions[i])
					}
				}
			}
		})
	}
}
