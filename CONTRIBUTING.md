# Contributing

Thank you for your interest in this project.

## Before you start

For a large change, open an issue first. This prevents wasted work.

## Rules

- Keep the code formatted with `gofmt`.
- Add a test for each change in behaviour.
- Make sure `go vet ./...` and `go test -race ./...` pass.
- Write one logical change per commit.
- Sign your commits.

## Local checks

```
gofmt -l .
go vet ./...
go test -race ./...
```

The integration test fetches one file over HTTP. To skip it, use
`go test -short ./...`.

## License

This project uses the Apache License 2.0. Your contribution goes out under the
same license. Do not remove the existing copyright headers. Add your own line
if you want.
