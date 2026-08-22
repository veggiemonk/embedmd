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
// The exit status follows diff(1): 0 when there is nothing to report, 1 when
// -d found a difference, and 2 when the run failed.
//
// For more information on the format of the commands, read the documentation
// of the github.com/veggiemonk/embedmd/embedmd package.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/veggiemonk/embedmd/embedmd"
)

// buildVersion is stamped by -ldflags when GoReleaser builds a release. A
// binary built any other way leaves it empty, and version reads the build
// information instead.
var buildVersion string

// version reports the version of the running binary. "go install
// module@version" records the version, and a build from a working tree
// records the commit and whether the tree was clean, so a binary can always
// say where it came from.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		if buildVersion == "" {
			return "unknown"
		}
		return buildVersion
	}
	return formatVersion(buildVersion, info)
}

func formatVersion(stamp string, info *debug.BuildInfo) string {
	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}

	v := stamp
	if v == "" {
		v = info.Main.Version
	}
	if v == "" || v == "(devel)" {
		v = "unknown"
		if revision != "" {
			v = revision[:min(len(revision), 12)]
		}
	}
	// Go already marks a pseudo-version from a changed tree with "+dirty".
	// Use the same mark when the version comes from somewhere else.
	if modified == "true" && !strings.HasSuffix(v, "+dirty") {
		v += "+dirty"
	}
	return v
}

// Exit status. The values follow diff(1), so a script can tell a difference
// from a failure.
const (
	exitOK        = 0
	exitDiffFound = 1
	exitError     = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop() // os.Exit runs no deferred call, so stop here.
	os.Exit(status)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("embedmd", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rewrite := flags.Bool("w", false, "write result to (markdown) file instead of stdout")
	doDiff := flags.Bool("d", false, "display diffs instead of rewriting files")
	printVersion := flags.Bool("v", false, "display embedmd version")
	flags.Usage = func() {
		// Nothing useful is left to do when a message cannot be printed.
		_, _ = fmt.Fprintln(stderr, "usage: embedmd [flags] [path ...]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK // -h asked for the usage and got it.
		}
		return exitError
	}

	if *printVersion {
		_, _ = fmt.Fprintln(stdout, "embedmd version: "+version())
		return exitOK
	}

	a := app{stdout: stdout, stderr: stderr}
	foundDiff, err := a.embed(ctx, flags.Args(), *rewrite, *doDiff)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitError
	}
	if foundDiff {
		return exitDiffFound
	}
	return exitOK
}

// app holds where a run writes. Tests supply their own writers, so the
// package keeps no writer of its own that a test would have to swap and put
// back.
type app struct {
	stdout io.Writer
	stderr io.Writer
}

func (a app) embed(ctx context.Context, paths []string, rewrite, doDiff bool) (foundDiff bool, err error) {
	if rewrite && doDiff {
		return false, errors.New("error: cannot use -w and -d simultaneously")
	}

	if len(paths) == 0 {
		return false, errors.New("error: no markdown files provided")
	}

	// One bad file does not hide the others: every path is tried, and every
	// failure is reported.
	var errs []error
	for _, path := range paths {
		d, err := a.processFile(ctx, path, rewrite, doDiff)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s:%w", path, err))
			continue
		}
		foundDiff = foundDiff || d
	}
	return foundDiff, errors.Join(errs...)
}

func (a app) processFile(ctx context.Context, path string, rewrite, doDiff bool) (foundDiff bool, err error) {
	if filepath.Ext(path) != ".md" {
		return false, fmt.Errorf("not a markdown file")
	}

	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	buf := new(bytes.Buffer)
	err = embedmd.Process(ctx, buf, bytes.NewReader(original), embedmd.WithBaseDir(filepath.Dir(path)))
	if err != nil {
		return false, err
	}

	if doDiff {
		data := diff(string(original), buf.String())
		if data == "" {
			return false, nil
		}
		if _, err := io.WriteString(a.stdout, data); err != nil {
			return true, err
		}
		return true, nil
	}

	if rewrite {
		if bytes.Equal(original, buf.Bytes()) {
			return false, nil // nothing changed, so leave the file alone.
		}
		return false, replaceFile(path, buf.Bytes())
	}

	_, err = io.Copy(a.stdout, buf)
	return false, err
}

// replaceFile writes data over path in one step. It writes a temporary file
// in the same directory, flushes it, and renames it over path, so an
// interrupted run leaves either the old file or the new one. Writing in place
// leaves a mixture of the two.
//
// A rename replaces the name, so a hard link to the old file keeps the old
// content. A symbolic link is followed, and the file it points at is the one
// replaced.
func replaceFile(path string, data []byte) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// filepath.Dir gives "." for a bare file name, where filepath.Split
	// gives "", which sends the temporary file to the system temporary
	// directory. A rename from there to here fails whenever the two sit on
	// different file systems.
	dir, name := filepath.Dir(path), filepath.Base(path)
	tmp, err := os.CreateTemp(dir, name+".tmp*")
	if err != nil {
		return err
	}
	// The remove does nothing once the rename below has succeeded.
	defer func() { _ = os.Remove(tmp.Name()) }()

	if err := writeAndClose(tmp, data, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// writeAndClose fills f and closes it, whatever happens.
func writeAndClose(f *os.File, data []byte, mode os.FileMode) (err error) {
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	// os.CreateTemp makes a file that only its owner can read.
	if err := f.Chmod(mode); err != nil {
		return err
	}
	return f.Sync()
}
