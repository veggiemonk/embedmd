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

// embedmd
//
// embedmd embeds files or fractions of files into markdown files.
// It does so by searching embedmd commands, which are a subset of the
// markdown syntax for comments. This means they are invisible when
// markdown is rendered, so they can be kept in the file as pointers
// to the origin of the embedded text.
//
// The command receives a list of markdown files to process. At least one
// file must be provided; reading from standard input is not supported.
//
// embedmd supports two flags:
// -d: will print the difference of the input file with what the output
//
//	would have been if executed.
//
// -w: rewrites the given files rather than writing the output to the standard
//
//	output.
//
// For more information on the format of the commands, read the documentation
// of the github.com/veggiemonk/embedmd/embedmd package.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/veggiemonk/embedmd/embedmd"
)

// modified while building by -ldflags.
var version = "unknown"

func usage() {
	fmt.Fprintf(os.Stderr, "usage: embedmd [flags] [path ...]\n")
	flag.PrintDefaults()
}

func main() {
	rewrite := flag.Bool("w", false, "write result to (markdown) file instead of stdout")
	doDiff := flag.Bool("d", false, "display diffs instead of rewriting files")
	printVersion := flag.Bool("v", false, "display embedmd version")
	flag.Usage = usage
	flag.Parse()

	if *printVersion {
		fmt.Println("embedmd version: " + version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	diff, err := embed(ctx, flag.Args(), *rewrite, *doDiff)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if diff && *doDiff {
		os.Exit(2)
	}
}

var stdout io.Writer = os.Stdout

func embed(ctx context.Context, paths []string, rewrite, doDiff bool) (foundDiff bool, err error) {
	if rewrite && doDiff {
		return false, fmt.Errorf("error: cannot use -w and -d simultaneously")
	}

	if len(paths) == 0 {
		return false, fmt.Errorf("error: no markdown files provided")
	}

	for _, path := range paths {
		d, err := processFile(ctx, path, rewrite, doDiff)
		if err != nil {
			return false, fmt.Errorf("%s:%v", path, err)
		}
		foundDiff = foundDiff || d
	}
	return foundDiff, nil
}

type file interface {
	io.ReadCloser
	io.WriterAt
	Truncate(int64) error
}

// replaced by testing functions.
var openFile = func(name string) (file, error) {
	return os.OpenFile(name, os.O_RDWR, 0666)
}

func processFile(ctx context.Context, path string, rewrite, doDiff bool) (foundDiff bool, err error) {
	if filepath.Ext(path) != ".md" {
		return false, fmt.Errorf("not a markdown file")
	}

	f, err := openFile(path)
	if err != nil {
		return false, err
	}
	// Report a failed close, but do not hide an earlier error.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var original bytes.Buffer
	var r io.Reader = f
	if doDiff {
		r = io.TeeReader(f, &original)
	}

	buf := new(bytes.Buffer)
	if err := embedmd.Process(ctx, buf, r, embedmd.WithBaseDir(filepath.Dir(path))); err != nil {
		return false, err
	}

	if doDiff {
		data, err := diff(original.String(), buf.String())
		if err != nil || len(data) == 0 {
			return false, err
		}
		fmt.Fprintf(stdout, "%s", data)
		return true, nil
	}

	if rewrite {
		n, err := f.WriteAt(buf.Bytes(), 0)
		if err != nil {
			return false, fmt.Errorf("could not write: %v", err)
		}
		return false, f.Truncate(int64(n))
	}

	_, err = io.Copy(stdout, buf)
	return false, err
}

func diff(a, b string) (string, error) {
	return difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:       difflib.SplitLines(a),
		B:       difflib.SplitLines(b),
		Context: 3,
	})
}
