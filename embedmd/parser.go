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
	"bufio"
	"fmt"
	"io"
	"strings"
)

// commandRunner runs one directive and writes its output. eol is the line
// terminator of the document, so that generated lines end like the lines
// around them.
type commandRunner func(w io.Writer, cmd *command, eol string) error

func process(out io.Writer, in io.Reader, run commandRunner) error {
	w := &errWriter{w: out}
	s := newLineScanner(in)

	state := parsingText
	for state != nil {
		next, err := state(w, s, run)
		if err == nil {
			err = w.err
		}
		if err != nil {
			return fmt.Errorf("%d: %w", s.line, err)
		}
		state = next
	}

	if err := s.readErr; err != nil {
		return fmt.Errorf("%d: %w", s.line, err)
	}
	return w.err
}

// errWriter remembers the first write error. Once one happens, later writes
// do nothing and report the same error, so a caller can check once at the end
// instead of after every write.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	n, err := e.w.Write(p)
	e.err = err
	return n, err
}

func (e *errWriter) writeString(s string) {
	if e.err != nil || s == "" {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

// writeLine writes a line and its terminator.
func (e *errWriter) writeLine(text, eol string) {
	e.writeString(text)
	e.writeString(eol)
}

// lineScanner reads one line at a time and keeps the line terminator, so that
// a rewrite leaves CRLF as CRLF. bufio.Scanner cannot do this: it strips the
// terminator, and it refuses a line longer than 64 KiB.
type lineScanner struct {
	r       *bufio.Reader
	text    string
	eol     string
	lastEol string // the last terminator seen, for a last line without one.
	line    int
	readErr error
}

func newLineScanner(r io.Reader) *lineScanner {
	return &lineScanner{r: bufio.NewReader(r)}
}

func (s *lineScanner) Scan() bool {
	if s.readErr != nil {
		return false
	}
	line, err := s.r.ReadString('\n')
	if err != nil {
		if err != io.EOF {
			s.readErr = err
			return false
		}
		if line == "" {
			return false // end of file, on a line boundary.
		}
		// The last line carries no terminator.
	}
	s.line++
	s.text, s.eol = splitEOL(line)
	if s.eol != "" {
		s.lastEol = s.eol
	}
	return true
}

func (s *lineScanner) Text() string { return s.text }

func (s *lineScanner) Eol() string { return s.eol }

func (s *lineScanner) DocumentEol() string {
	if s.eol != "" {
		return s.eol
	}
	if s.lastEol != "" {
		return s.lastEol
	}
	return "\n"
}

// splitEOL separates a line from its terminator. The terminator is empty when
// the last line of the input carries none.
func splitEOL(line string) (text, eol string) {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return line[:len(line)-2], "\r\n"
	case strings.HasSuffix(line, "\n"):
		return line[:len(line)-1], "\n"
	default:
		return line, ""
	}
}

type textScanner interface {
	Scan() bool
	// Text returns the current line without its terminator.
	Text() string
	// Eol returns the terminator of the current line: "\n", "\r\n", or ""
	// for a last line that carries none.
	Eol() string
	// DocumentEol returns the terminator to give a line that needs one: the
	// terminator of the current line, or the last one seen in the file.
	DocumentEol() string
}

type state func(*errWriter, textScanner, commandRunner) (state, error)

func parsingText(out *errWriter, s textScanner, run commandRunner) (state, error) {
	if !s.Scan() {
		return nil, nil // end of file, which is fine.
	}
	switch line := s.Text(); {
	case strings.HasPrefix(line, "[embedmd]:#"):
		return parsingCmd, nil
	case strings.HasPrefix(line, "```"):
		return codeParser{print: true}.parse, nil
	default:
		out.writeLine(line, s.Eol())
		return parsingText, nil
	}
}

func parsingCmd(out *errWriter, s textScanner, run commandRunner) (state, error) {
	line := s.Text()
	// A directive on a last line with no terminator still needs one, because
	// the generated block has to start on a line of its own. It takes the
	// terminator the rest of the file uses.
	eol := s.DocumentEol()
	out.writeLine(line, eol)

	_, args, _ := strings.Cut(line, "#")
	cmd, err := parseCommand(args)
	if err != nil {
		return nil, err
	}
	if err := run(out, cmd, eol); err != nil {
		return nil, err
	}
	if !s.Scan() {
		return nil, nil // end of file, which is fine.
	}
	if strings.HasPrefix(s.Text(), "```") {
		return codeParser{print: false}.parse, nil
	}
	out.writeLine(s.Text(), s.Eol())
	return parsingText, nil
}

type codeParser struct{ print bool }

func (c codeParser) parse(out *errWriter, s textScanner, run commandRunner) (state, error) {
	if c.print {
		out.writeLine(s.Text(), s.Eol())
	}
	if !s.Scan() {
		return nil, fmt.Errorf("unbalanced code section")
	}
	if !strings.HasPrefix(s.Text(), "```") {
		return c.parse, nil
	}

	// print the end of the code section if needed and go back to parsing text.
	if c.print {
		out.writeLine(s.Text(), s.Eol())
	}
	return parsingText, nil
}
