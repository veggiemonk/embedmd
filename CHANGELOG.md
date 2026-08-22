# Changelog

All notable changes to this project are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

This is the first release of the fork. The project moved from
`github.com/campoy/embedmd` to `github.com/veggiemonk/embedmd`, because
upstream stopped taking changes.

### Added

- Directive option `!` before a regular expression. The matching line acts as a
  boundary but stays out of the output.
- Directive option `dedent`. It removes the common leading whitespace.
- Directive option `trim`. It removes trailing blank lines.
- Directive option `s/old/new/`. It replaces text in the extracted content.
- `Process` and `Fetcher.Fetch` take a `context.Context`. The HTTP fetch obeys
  cancellation and deadlines.
- The command cancels its work on SIGINT and SIGTERM.
- Pre-built binaries for Linux, macOS, and Windows, built by GoReleaser.
- `WithHTTPClient`, to supply the `http.Client` that URLs are fetched with.
- An exit status of its own for "`-d` found a difference".

### Changed

- The module path is `github.com/veggiemonk/embedmd`. **Breaking.**
- `Process` and `Fetcher.Fetch` have a new first parameter. **Breaking.**
- The `go` directive moved from 1.11 to 1.27. CI tests Go 1.27.
- Errors wrap with `%w`, so callers can inspect them.
- The integration test builds the binary under test from source, instead of
  running whichever binary is on `PATH`.
- CI runs on push and on pull request, on Linux, macOS, and Windows, with
  `go vet`, `-race`, `gofmt`, and golangci-lint.
- Every changed file carries a fork copyright line and a change notice, as
  Apache License 2.0 section 4(b) asks for. `NOTICE` lists the upstream
  contributors.
- The exit status follows `diff(1)`: 0 for nothing to report, 1 for a
  difference found by `-d`, 2 for a failure. Status 2 used to mean both of the
  last two. **Breaking** for a script that tests for status 2 after `-d`.
- `-w` writes a temporary file and renames it over the file, instead of
  writing over the file in place. It writes nothing when the result equals the
  file on disk.
- `-w` keeps the line terminator of each line and adds none of its own, so a
  CRLF file stays a CRLF file, and a file with no terminator on the last line
  keeps none.
- A run processes every file named on the command line and reports every
  failure, instead of stopping at the first one.
- `-v` reports the version from the build information when the binary carries
  no build stamp, so a binary from `go install` gives a real answer.
- The unified diff is computed by this project. It is byte for byte what
  `diff -u` gives, including the mark for a missing terminator on a last line.
- `Process` no longer confines a path itself. The built-in fetcher does it, so
  a custom `Fetcher` sets its own boundary. **Breaking** for a caller that
  relied on `Process` to check the path of a custom fetcher.

### Fixed

- A crash on `[embedmd]:# (file.go go $)`. A bare `$` in the first range slot
  gave a nil pointer dereference. It is now refused with a message.
- A line longer than 64 KiB stopped the run with
  `bufio.Scanner: token too long`, and reported it against line 0.
- Every write is checked. A failed write used to pass unnoticed.
- A typo in a copyright line: "Google Intt." became "Google Inc." in
  `main_test.go`.

### Removed

- The dependency on `pmezard/go-difflib`. The module now has no dependencies.
- The hand-written `releases/release.sh` script. GoReleaser replaces it.
- The upstream tags `v1.0.0` and `v2.0.0`. The upstream `v2.0.0` release was
  broken, because the module path had no `/v2` suffix.

### Security

- A symbolic link inside the base directory that pointed out of it was
  followed and read. The path check compared cleaned strings and never
  resolved links. The read now goes through `os.OpenInRoot`, which refuses
  `..`, an absolute path, and a link that leaves the base directory.
- An HTTP response above the 10 MiB limit was truncated to exactly the limit,
  with no error, and the cut content went into the document as if it were
  whole. It is now an error.

The fork also carries these fixes, which came before the fork point:

- A path traversal fix in the local file fetch.
- A timeout and a response size limit on HTTP fetches.

## Before the fork

For the history of the original project, see
https://github.com/campoy/embedmd.
