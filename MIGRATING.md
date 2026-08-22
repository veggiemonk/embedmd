# Migrating from campoy/embedmd

This fork keeps the command name, the flags, and the directive syntax. For most
users the change is one line.

## Command-line users

Install from the new path:

```
go install github.com/veggiemonk/embedmd@latest
```

The binary is still called `embedmd`, so it replaces the old one in
`$GOPATH/bin`.

## GitHub Actions and other CI

Replace the install step:

```diff
-      - run: go install github.com/campoy/embedmd@latest
+      - run: go install github.com/veggiemonk/embedmd@latest
```

## Library users

Change the import path:

```diff
-import "github.com/campoy/embedmd/embedmd"
+import "github.com/veggiemonk/embedmd/embedmd"
```

`Process` now takes a `context.Context` as its first argument:

```diff
-err := embedmd.Process(out, in, embedmd.WithBaseDir(dir))
+err := embedmd.Process(ctx, out, in, embedmd.WithBaseDir(dir))
```

If you supply your own `Fetcher`, add the context to your `Fetch` method:

```diff
-func (f myFetcher) Fetch(dir, path string) ([]byte, error) {
+func (f myFetcher) Fetch(ctx context.Context, dir, path string) ([]byte, error) {
```

If you supply your own `Fetcher`, it is also responsible for its own limits.
`Process` used to check that a path stayed inside the base directory before it
called `Fetch`. That check now lives in the built-in fetcher.

## Behaviour that changed in this fork

- `-d` exits with status 1 when it finds a difference, and status 2 when the
  run fails. Both used to be status 2. A script that tests for status 2 after
  `-d` needs one line changed.
- `-w` keeps the line terminators of the file. A CRLF file used to become an
  LF file, and a file with no terminator on the last line used to get one.
- A bare `$` as the first range argument, as in `[embedmd]:# (f.go go $)`, is
  refused. It used to crash the tool.

## Behaviour that changed before the fork

`embedmd` no longer reads from the standard input. Give it at least one file.

## Version numbers

The fork starts again at `v1.0.0`. The tags `v1.0.0` and `v2.0.0` from the
original repository are not part of this fork. The upstream `v2.0.0` tag was
broken: the module path had no `/v2` suffix, so the Go tool could not use it.
