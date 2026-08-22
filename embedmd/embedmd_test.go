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
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/veggiemonk/embedmd/internal/testutil"
)

const content = `
package main

import "fmt"

func main() {
        fmt.Println("hello, test")
}
`

func TestExtract(t *testing.T) {
	tc := []struct {
		name       string
		start, end string
		out        string
		err        string
	}{
		{
			name: "no limits",
			out:  string(content),
		},
		{
			name:  "only one line",
			start: "/func main.*\n/", out: "func main() {\n",
		},
		{
			name:  "from package to end",
			start: "/package main/", end: "$", out: string(content[1:]),
		},
		{
			name:  "not matching",
			start: "/gopher/", err: "could not match \"/gopher/\"",
		},
		{
			name:  "part of a line",
			start: "/fmt.P/", end: "/hello/", out: "fmt.Println(\"hello",
		},
		{
			name:  "function call",
			start: "/fmt\\.[^()]*/", out: "fmt.Println",
		},
		{
			name:  "from fmt to end of line",
			start: "/fmt.P.*\n/", out: "fmt.Println(\"hello, test\")\n",
		},
		{
			name:  "from func to end of next line",
			start: "/func/", end: "/Println.*\n/", out: "func main() {\n        fmt.Println(\"hello, test\")\n",
		},
		{
			name:  "from func to }",
			start: "/func main/", end: "/}/", out: "func main() {\n        fmt.Println(\"hello, test\")\n}",
		},

		{
			name:  "bad start regexp",
			start: "/(/", err: "error parsing regexp: missing closing ): `(`",
		},
		{
			name:  "bad regexp",
			start: "something", err: "missing slashes (/) around \"something\"",
		},
		{
			name:  "bad end regexp",
			start: "/fmt.P/", end: "/)/", err: "error parsing regexp: unexpected ): `)`",
		},

		{
			name:  "start and end of line ^$",
			start: "/^func main/", end: "/}$/", out: "func main() {\n        fmt.Println(\"hello, test\")\n}",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			b, err := extract([]byte(content), tt.start, tt.end)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}
			if string(b) != tt.out {
				t.Errorf("case [%s]: expected extracting %q; got %q", tt.name, tt.out, b)
			}
		})
	}
}

func TestExtractFromFile(t *testing.T) {
	tc := []struct {
		name    string
		cmd     command
		baseDir string
		files   map[string][]byte
		out     string
		err     string
	}{
		{
			name:  "extract the whole file",
			cmd:   command{path: "code.go", lang: "go"},
			files: map[string][]byte{"code.go": []byte(content)},
			out:   "```go\n" + string(content) + "```\n",
		},
		{
			name:    "extract the whole from a different directory",
			cmd:     command{path: "code.go", lang: "go"},
			baseDir: "sample",
			files:   map[string][]byte{"sample/code.go": []byte(content)},
			out:     "```go\n" + string(content) + "```\n",
		},
		{
			name:  "added line break",
			cmd:   command{path: "code.go", lang: "go", start: "/fmt\\.Println/"},
			files: map[string][]byte{"code.go": []byte(content)},
			out:   "```go\nfmt.Println\n```\n",
		},
		{
			name: "missing file",
			cmd:  command{path: "code.go", lang: "go"},
			err:  "could not read code.go: file does not exist",
		},
		{
			name:  "unmatched regexp",
			cmd:   command{path: "code.go", lang: "go", start: "/potato/"},
			files: map[string][]byte{"code.go": []byte(content)},
			err:   "could not extract content from code.go: could not match \"/potato/\"",
		},
		{
			name: "exclude start and end lines",
			cmd: command{
				path: "code.go", lang: "go",
				start: "/\\/\\/ start/", end: "/\\/\\/ end/",
				excludeStart: true, excludeEnd: true,
			},
			files: map[string][]byte{"code.go": []byte("// start\nfmt.Println(\"hello\")\n// end\n")},
			out:   "```go\nfmt.Println(\"hello\")\n```\n",
		},
		{
			name: "dedent strips common indent",
			cmd: command{
				path: "code.go", lang: "go",
				start: "/\\/\\/ start/", end: "/\\/\\/ end/",
				excludeStart: true, excludeEnd: true, dedent: true,
			},
			files: map[string][]byte{"code.go": []byte("// start\n\tfmt.Println(\"hello\")\n\tx := 1\n// end\n")},
			out:   "```go\nfmt.Println(\"hello\")\nx := 1\n```\n",
		},
		{
			name: "trim removes trailing blank lines",
			cmd: command{
				path: "code.go", lang: "go",
				start: "/\\/\\/ start/", end: "/\\/\\/ end/",
				excludeStart: true, excludeEnd: true, trim: true,
			},
			files: map[string][]byte{"code.go": []byte("// start\nline1\n\n\n// end\n")},
			out:   "```go\nline1\n```\n",
		},
		{
			name: "substitution replaces text",
			cmd: command{
				path: "code.go", lang: "go",
				substitutions: []substitution{{old: "ELLIPSIS", new: "..."}},
			},
			files: map[string][]byte{"code.go": []byte("x = ELLIPSIS\n")},
			out:   "```go\nx = ...\n```\n",
		},
		{
			name: "all transforms combined",
			cmd: command{
				path: "code.go", lang: "go",
				start: "/\\/\\/ snippet-start/", end: "/\\/\\/ snippet-end/",
				excludeStart: true, excludeEnd: true, dedent: true, trim: true,
				substitutions: []substitution{{old: "ELLIPSIS", new: "..."}},
			},
			files: map[string][]byte{"code.go": []byte("// snippet-start\n\ttokenSource := ELLIPSIS\n\tclient := New(ctx, tokenSource)\n\n// snippet-end\n")},
			out:   "```go\ntokenSource := ...\nclient := New(ctx, tokenSource)\n```\n",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			e := embedder{
				baseDir: tt.baseDir,
				Fetcher: fakeFileProvider(tt.files),
			}

			w := new(bytes.Buffer)
			err := e.runCommand(context.Background(), w, &tt.cmd)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}
			if w.String() != tt.out {
				t.Errorf("case [%s]: expected output\n%q\n; got \n%q\n", tt.name, tt.out, w.String())
			}
		})
	}
}

type fakeFileProvider map[string][]byte

func (c fakeFileProvider) Fetch(_ context.Context, dir, path string) ([]byte, error) {
	if f, ok := c[filepath.Join(dir, path)]; ok {
		return f, nil
	}
	return nil, os.ErrNotExist
}

func TestProcess(t *testing.T) {
	tc := []struct {
		name  string
		in    string
		dir   string
		files map[string][]byte
		urls  map[string][]byte
		out   string
		err   string
		diff  bool
	}{
		{
			name: "missing file",
			in: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"Yay!\n",
			err: "2: could not read code.go: file does not exist",
		},
		{
			name: "generating code for first time",
			in: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"Yay!\n",
			files: map[string][]byte{"code.go": []byte(content)},
			out: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"```go\n" +
				string(content) +
				"```\n" +
				"Yay!\n",
		},
		{
			name: "generating code for first time with base dir",
			dir:  "sample",
			in: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"Yay!\n",
			files: map[string][]byte{"sample/code.go": []byte(content)},
			out: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"```go\n" +
				string(content) +
				"```\n" +
				"Yay!\n",
		},
		{
			name: "replacing existing code",
			in: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"```go\n" +
				string(content) +
				"```\n" +
				"Yay!\n",
			files: map[string][]byte{"code.go": []byte(content)},
			out: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"```go\n" +
				string(content) +
				"```\n" +
				"Yay!\n",
		},
		{
			name: "embedding code from a URL",
			in: "# This is some markdown\n" +
				"[embedmd]:# (https://fakeurl.com/main.go)\n" +
				"Yay!\n",
			urls: map[string][]byte{"https://fakeurl.com/main.go": []byte(content)},
			out: "# This is some markdown\n" +
				"[embedmd]:# (https://fakeurl.com/main.go)\n" +
				"```go\n" +
				string(content) +
				"```\n" +
				"Yay!\n",
		},
		{
			name: "embedding code from a URL not found",
			in: "# This is some markdown\n" +
				"[embedmd]:# (https://fakeurl.com/main.go)\n" +
				"Yay!\n",
			err: "2: could not read https://fakeurl.com/main.go: status Not Found",
		},
		{
			name: "embedding code from a bad URL",
			in: "# This is some markdown\n" +
				"[embedmd]:# (https://fakeurl.com\\main.go)\n" +
				"Yay!\n",
			err: "2: could not read https://fakeurl.com\\main.go: parse \"https://fakeurl.com\\\\main.go\": invalid character \"\\\\\" in host name",
		},
		{
			name: "normal relative path within base dir still works",
			dir:  "sample",
			in: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"Yay!\n",
			files: map[string][]byte{"sample/code.go": []byte(content)},
			out: "# This is some markdown\n" +
				"[embedmd]:# (code.go)\n" +
				"```go\n" +
				string(content) +
				"```\n" +
				"Yay!\n",
		},
		{
			name: "dollar as the only range argument",
			in: "# This is some markdown\n" +
				"[embedmd]:# (code.go go $)\n",
			files: map[string][]byte{"code.go": []byte(content)},
			err:   "2: $ must follow a start regexp",
		},
		{
			name: "ignore commands in code blocks",
			in: "# This is some markdown\n" +
				"```markdown\n" +
				"[embedmd]:# (nothing.md)\n" +
				"```\n" +
				"Yay!\n",
			out: "# This is some markdown\n" +
				"```markdown\n" +
				"[embedmd]:# (nothing.md)\n" +
				"```\n" +
				"Yay!\n",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cp := mixedContentProvider{tt.files, tt.urls}
			if tt.diff {
				cp.files["file.md"] = []byte(tt.in)
			}
			opts := []Option{WithFetcher(cp)}
			if tt.dir != "" {
				opts = append(opts, WithBaseDir(tt.dir))
			}
			err := Process(context.Background(), &out, strings.NewReader(tt.in), opts...)
			if !testutil.EqErr(t, tt.name, err, tt.err) {
				return
			}
			if tt.out != out.String() {
				t.Errorf("case [%s]: expected output:\n###\n%s\n###; got###\n%s\n###", tt.name, tt.out, out.String())
			}
		})
	}
}

type mixedContentProvider struct {
	files, urls map[string][]byte
}

func (c mixedContentProvider) Fetch(_ context.Context, dir, path string) ([]byte, error) {
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		resolved := filepath.Join(dir, filepath.FromSlash(path))
		if f, ok := c.files[resolved]; ok {
			return f, nil
		}
		return nil, os.ErrNotExist
	}

	_, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	if b, ok := c.urls[path]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("status Not Found")
}
