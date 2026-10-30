# Contributing

Use Go 1.26+ and preserve the small, anonymous terminal-only scope. Open an issue
for substantial changes. Never commit credentials or personal connection logs.

Before a pull request, run `gofmt -w cmd internal`, `go test ./...`,
`go test -race ./...` and `go vet ./...`. Build both commands for Linux and
Windows as described in the README. Add deterministic tests for behavior changes,
using dynamic ports and bounded timeouts. Keep server ownership of game state.

Test the three-terminal local workflow when changing networking or the UI.
Describe what you tested, your OS/terminal, and any behavior you could not verify.
Do not claim Windows terminal compatibility solely from cross-compilation.

Maintainers can publish a release by pushing a reviewed `v*` tag. The release
workflow runs validation before uploading the four native binaries and checksums.
