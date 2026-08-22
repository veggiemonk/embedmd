# Security policy

## Supported versions

The latest release gets security fixes.

## How to report

Do not open a public issue for a security problem.

Use GitHub private vulnerability reporting:
https://github.com/veggiemonk/embedmd/security/advisories/new

Please include the version, the steps to reproduce, and the effect.

## Response

You get an acknowledgement within 7 days. This is a small project, so a fix can
take longer.

## Notes on the threat model

`embedmd` reads Markdown files and embeds content from local paths and from
URLs. Both inputs come from the file it processes.

- A local path cannot escape the base directory. The base directory is the
  directory of the Markdown file. The read goes through `os.OpenInRoot`, so a
  `..` path, an absolute path, and a symbolic link that points out of the base
  directory are all refused.
- This holds for the built-in fetcher. A program that supplies its own
  `Fetcher` with `WithFetcher`, or that calls `Process` without
  `WithBaseDir`, has to set its own boundary.
- An HTTP fetch has a 10 second timeout and a 10 MiB response limit.
- The tool follows any URL that a directive gives. Do not run `embedmd` on
  Markdown from a source you do not trust.
