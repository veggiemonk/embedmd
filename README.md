[![Build and Test](https://github.com/veggiemonk/embedmd/actions/workflows/ci.yaml/badge.svg)](https://github.com/veggiemonk/embedmd/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/veggiemonk/embedmd.svg)](https://pkg.go.dev/github.com/veggiemonk/embedmd)
[![Go Report Card](https://goreportcard.com/badge/github.com/veggiemonk/embedmd)](https://goreportcard.com/report/github.com/veggiemonk/embedmd)

# embedmd

> **This is a maintained fork of [campoy/embedmd](https://github.com/campoy/embedmd).**
> The original project no longer takes changes, so this fork continues the work.
> It contains security fixes and features that upstream does not have.
> See [MIGRATING.md](MIGRATING.md) if you come from the original tool.

Are you tired of copy pasting your code into your `README.md` file, just to
forget about it later on and have unsynced copies? Or even worse, code
that does not even compile?

Then `embedmd` is for you!

`embedmd` embeds files or fractions of files into Markdown files. It does
so by searching `embedmd` commands, which are a subset of the Markdown
syntax for comments. This means they are invisible when Markdown is
rendered, so they can be kept in the file as pointers to the origin of
the embedded text.

The command receives a list of Markdown files. You must give at least one file.
The command does not read from the standard input.

The format of an `embedmd` command is:

```Markdown
[embedmd]:# (pathOrURL language /start regexp/ /end regexp/)
```

The embedded code will be extracted from the file at `pathOrURL`,
which can either be a relative path to a file in the local file
system (using always forward slashes as directory separator) or
a URL starting with `http://` or `https://`.
If the `pathOrURL` is a URL the tool will fetch the content in that URL.
The embedded content starts at the first line that matches `/start regexp/`
and finishes at the first line matching `/end regexp/`.

Omitting the the second regular expression will embed only the piece of text
that matches `/regexp/`:

```Markdown
[embedmd]:# (pathOrURL language /regexp/)
```

To embed the whole line matching a regular expression you can use:

```Markdown
[embedmd]:# (pathOrURL language /.*regexp.*/)
```

To embed from a point to the end you should use:

```Markdown
[embedmd]:# (pathOrURL language /start regexp/ $)
```

To embed a whole file, omit both regular expressions:

```Markdown
[embedmd]:# (pathOrURL language)
```

You can omit the language in any of the previous commands, and the extension
of the file will be used for the snippet syntax highlighting.

This works when the file extensions matches the name of the language (like Go
files, since `.go` matches `go`). However, this will fail with other files like
`.md` whose language name is `markdown`.

```Markdown
[embedmd]:# (file.ext)
```

## Options

Options can be added after the regular expressions in an `embedmd` command.

### Excluding delimiter lines

Prefix a regexp with `!` to use it as a boundary but exclude the matching line
from the output. This is useful when you use comments in your code to mark
snippet boundaries:

```go
// snippet-start
fmt.Println("hello")
// snippet-end
```

```Markdown
[embedmd]:# (example.go go !/\/\/ snippet-start/ !/\/\/ snippet-end/)
```

This embeds only `fmt.Println("hello")`, excluding the marker comments.

You can also exclude just the start or just the end:

```Markdown
[embedmd]:# (example.go go !/\/\/ start/ /end/)
```

### Stripping indentation

Code inside functions is typically indented. The `dedent` option removes the
common leading whitespace from all lines, so your Markdown code blocks are not
unnecessarily indented:

```Markdown
[embedmd]:# (example.go go !/\/\/ snippet-start/ !/\/\/ snippet-end/ dedent)
```

### Trimming trailing blank lines

Go's `gofmt` inserts a blank line between code and a following comment. The
`trim` option removes trailing blank lines from the extracted content:

```Markdown
[embedmd]:# (example.go go !/\/\/ snippet-start/ !/\/\/ snippet-end/ trim)
```

### Text substitution

The `s/old/new/` option replaces all occurrences of `old` with `new` in the
extracted content. This lets you use placeholder tokens in compilable code:

```Markdown
[embedmd]:# (example.go go /func demo/ /}/ s/ELLIPSIS/.../)
```

Multiple substitutions can be chained:

```Markdown
[embedmd]:# (example.go go /func/ /}/ s/ELLIPSIS/.../ s/_ = ELLIPSIS/.../)
```

### Combining options

Options can be combined. They are applied in this order: line exclusion, trailing
blank line trimming, dedentation, then text substitution.

```Markdown
[embedmd]:# (example.go go !/\/\/ snippet-start/ !/\/\/ snippet-end/ dedent trim s/ELLIPSIS/.../)
```

## Installation

> You can install Go by following [these instructions](https://golang.org/doc/install).

`embedmd` is written in Go, so if you have Go installed you can install it with
`go install`:

```
go install github.com/veggiemonk/embedmd@latest
```

This will download the code, compile it, and leave an `embedmd` binary
in `$GOPATH/bin`.

The binary keeps the name `embedmd`. If you installed the original tool, the
new binary replaces it in `$GOPATH/bin`.

Pre-built binaries for Linux, macOS, and Windows are attached to each
[release](https://github.com/veggiemonk/embedmd/releases).

## Usage:

Given the two files in [sample](sample):

*hello.go:*

[embedmd]:# (sample/hello.go)
```go
// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Hello, there, it is", time.Now())
}
```

*docs.md:*

[embedmd]:# (sample/docs.md Markdown /./ /embedmd.*time.*/)
```Markdown
# A hello world in Go

Go is very simple, here you can see a whole "hello, world" program.

[embedmd]:# (hello.go)

We can try to embed a file from a directory.

[embedmd]:# (test/hello.go /func main/ $)

You always start with a `package` statement like:

[embedmd]:# (hello.go /package.*/)

Followed by an `import` statement:

[embedmd]:# (hello.go /import/ /\)/)

You can also see how to get the current time:

[embedmd]:# (hello.go /time\.[^)]*\)/)
```

# Flags

* `-w`: Executing `embedmd -w docs.md` will modify `docs.md`
and add the corresponding code snippets, as shown in
[sample/result.md](sample/result.md).

* `-d`: Executing `embedmd -d docs.md` will display the difference
between the contents of `docs.md` and the output of
`embedmd docs.md`.

## Use as a library

The `embedmd` package exposes `Process`. It takes a context, so you can cancel
a run and stop any HTTP fetch in flight.

```go
err := embedmd.Process(ctx, out, in, embedmd.WithBaseDir(dir))
```

See the [package documentation](https://pkg.go.dev/github.com/veggiemonk/embedmd/embedmd).

## License and origin

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE).

The original code was created by Francesc Campoy, first at Google Inc., with
other contributors. See [NOTICE](NOTICE) for the full attribution and for the
list of changes that this fork made.

Copyright 2016 Google Inc.  
Copyright 2026 Julien Bisconti and the embedmd fork contributors.

This is not an official Google product, and it is not related to the original
`campoy/embedmd` project.
