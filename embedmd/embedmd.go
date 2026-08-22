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

// Package embedmd provides a single function, Process, that parses markdown
// searching for markdown comments.
//
// The format of an embedmd command is:
//
//	[embedmd]:# (pathOrURL language /start regexp/ /end regexp/)
//
// The embedded code will be extracted from the file at pathOrURL,
// which can either be a relative path to a file in the local file
// system (using always forward slashes as directory separator) or
// a url starting with http:// or https://.
// If the pathOrURL is a url the tool will fetch the content in that url.
// The embedded content starts at the first line that matches /start regexp/
// and finishes at the first line matching /end regexp/.
//
// Omitting the the second regular expression will embed only the piece of
// text that matches /regexp/:
//
//	[embedmd]:# (pathOrURL language /regexp/)
//
// To embed the whole line matching a regular expression you can use:
//
//	[embedmd]:# (pathOrURL language /.*regexp.*\n/)
//
// If you want to embed from a point to the end you should use:
//
//	[embedmd]:# (pathOrURL language /start regexp/ $)
//
// Finally you can embed a whole file by omitting both regular expressions:
//
//	[embedmd]:# (pathOrURL language)
//
// You can ommit the language in any of the previous commands, and the extension
// of the file will be used for the snippet syntax highlighting. Note that while
// this works Go files, since the file extension .go matches the name of the language
// go, this will fail with other files like .md whose language name is markdown.
//
//	[embedmd]:# (file.ext)
package embedmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

// Process reads markdown from the given io.Reader searching for an embedmd
// command. When a command is found, it is executed and the output is written
// into the given io.Writer with the rest of standard markdown.
//
// The given context is passed to the Fetcher and cancels any content still to
// be fetched, including HTTP requests in flight.
func Process(ctx context.Context, out io.Writer, in io.Reader, opts ...Option) error {
	// One client for the whole run, so several URL directives share the
	// connection pool.
	e := embedder{Fetcher: fetcher{client: &http.Client{Timeout: httpTimeout}}}
	for _, opt := range opts {
		opt.f(&e)
	}
	run := func(w io.Writer, cmd *command, eol string) error { return e.runCommand(ctx, w, cmd, eol) }
	return process(out, in, run)
}

// An Option provides a way to adapt the Process function to your needs.
type Option struct{ f func(*embedder) }

// WithBaseDir indicates that the given path should be used to resolve relative
// paths.
//
// The default Fetcher also treats the base directory as a boundary: a
// directive cannot read a file out of it, not even through a symbolic link.
// Without a base directory there is no boundary, so do not process untrusted
// markdown without one.
func WithBaseDir(path string) Option {
	return Option{func(e *embedder) { e.baseDir = path }}
}

// WithFetcher provides a custom Fetcher to be used whenever a path or url needs
// to be fetched.
func WithFetcher(c Fetcher) Option {
	return Option{func(e *embedder) { e.Fetcher = c }}
}

// WithHTTPClient sets the http.Client that the default Fetcher uses for URLs.
// Supply one to change the timeout, the transport, or the redirect policy.
//
// It replaces the Fetcher, so a later WithFetcher overrides it, and it
// overrides an earlier WithFetcher.
func WithHTTPClient(c *http.Client) Option {
	return Option{func(e *embedder) { e.Fetcher = fetcher{client: c} }}
}

type embedder struct {
	Fetcher
	baseDir string
}

func (e *embedder) runCommand(ctx context.Context, w io.Writer, cmd *command, eol string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := e.Fetch(ctx, e.baseDir, cmd.path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", cmd.path, err)
	}

	content, offset, err := extract(b, cmd.start, cmd.end)
	if err != nil {
		return fmt.Errorf("could not extract content from %s: %w", cmd.path, err)
	}

	b = content

	// Apply transforms in order: exclude → trim → dedent → substitute.
	b = excludeLines(b, cmd.excludeStart, cmd.excludeEnd)
	if cmd.trim {
		b = trimTrailingBlankLines(b)
	}
	if cmd.dedent {
		// A start regexp can match in the middle of a line, and then the first
		// line of the content carries none of its own indentation. dedent has to
		// leave that line out of the comparison, or it finds no common prefix and
		// does nothing at all. Excluding the start line removes the problem with
		// the line.
		wholeFirstLine := cmd.excludeStart || offset == 0 || b[offset-1] == '\n'
		b = dedentBytes(b, wholeFirstLine)
	}
	b = applySubstitutions(b, cmd.substitutions)

	// The content may stop in the middle of a line, when a regexp matched
	// there. Close that line the way the document closes its lines, or a
	// CRLF document ends up with one LF line inside the block. A regexp such
	// as /package.*/ over a CRLF file stops on the CR itself, which is half
	// of a terminator, so drop it before adding a whole one.
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(bytes.TrimSuffix(b, []byte("\r")), eol...)
	}

	if _, err := io.WriteString(w, "```"+cmd.lang+eol); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = io.WriteString(w, "```"+eol)
	return err
}

// extract returns the part of b delimited by the start and end regexps, and
// the offset in b where that part begins.
//
// An empty start or end means the boundary is absent: parseCommand never
// produces an empty one. The end may also be "$", which means "to the end of
// the content".
func extract(b []byte, start, end string) ([]byte, int, error) {
	if start == "" && end == "" {
		return b, 0, nil
	}

	match := func(s string) ([]int, error) {
		if len(s) <= 2 || s[0] != '/' || s[len(s)-1] != '/' {
			return nil, fmt.Errorf("missing slashes (/) around %q", s)
		}
		re, err := regexp.CompilePOSIX(s[1 : len(s)-1])
		if err != nil {
			return nil, err
		}
		loc := re.FindIndex(b)
		if loc == nil {
			return nil, fmt.Errorf("could not match %q", s)
		}
		return loc, nil
	}

	offset := 0
	if start != "" {
		loc, err := match(start)
		if err != nil {
			return nil, 0, err
		}
		if end == "" {
			return b[loc[0]:loc[1]], loc[0], nil
		}
		offset = loc[0]
		b = b[offset:]
	}

	if end != "$" {
		loc, err := match(end)
		if err != nil {
			return nil, 0, err
		}
		b = b[:loc[1]]
	}

	return b, offset, nil
}
