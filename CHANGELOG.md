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

### Changed

- The module path is `github.com/veggiemonk/embedmd`. **Breaking.**
- `Process` and `Fetcher.Fetch` have a new first parameter. **Breaking.**
- The `go` directive moved from 1.11 to 1.24.
- Errors wrap with `%w`, so callers can inspect them.
- The integration test builds the binary under test from source, instead of
  running whichever binary is on `PATH`.
- CI runs on push and on pull request, on Linux, macOS, and Windows, with
  `go vet`, `-race`, `gofmt`, and golangci-lint.

### Removed

- The hand-written `releases/release.sh` script. GoReleaser replaces it.
- The upstream tags `v1.0.0` and `v2.0.0`. The upstream `v2.0.0` release was
  broken, because the module path had no `/v2` suffix.

### Security

The fork carries these fixes, which came before the fork point:

- A path traversal fix in the local file fetch.
- A timeout and a response size limit on HTTP fetches.

## Before the fork

For the history of the original project, see
https://github.com/campoy/embedmd.
