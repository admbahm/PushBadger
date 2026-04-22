# Contributing

## Build and test

```sh
make build   # compile to ./pushbadger
make test    # go test ./...
make lint    # go vet ./...
make clean   # remove ./pushbadger binary
```

The test suite requires `git` to be on your PATH (it creates real temporary
repositories). Run `go test ./test/integration/` to target integration tests
only.

## Filing issues and PRs

- Bugs and feature requests → GitHub Issues
- Small, focused fixes → PRs are welcome
- Larger changes → open an issue first to align on scope before writing code

## Licensing

By submitting a pull request you agree that your contribution is licensed
under the Apache 2.0 license, the same license as this project.

## AI-assisted development

This project was developed with AI assistance (Claude Code) and that's fine.
Contributions written with AI assistance are welcome — what matters is that the
code is correct, tested, and that you understand what you're submitting.
