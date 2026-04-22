# PushBadger

PushBadger analyzes git diffs and maps changed files to risk areas using
deterministic, path-based heuristics. No AI — just fast, reproducible signal
about which parts of your codebase are touched by a change.

## Requirements

- Go 1.24+
- git

## Install

**Build from source:**

```sh
git clone <repo-url>
cd pushbadger
make build          # produces ./pushbadger
```

Or without Make:

```sh
go build -o pushbadger ./cmd/pushbadger
```

> A versioned binary release is planned. Until then, build from source.
> `go install` requires the module to be published at a public module path,
> which has not been done yet.

## Usage

```
pushbadger analyze [flags]

Flags:
  --base string    Base ref for diff (default: auto-detected)
  --head string    Head ref for diff (default: HEAD)
  --staged         Diff staged changes  (HEAD → index)
  --working        Diff unstaged changes (index → worktree)
  --format string  Output format: text or json (default: text)
  --rules string   Path to custom rules YAML file
```

**Diff modes** — pick at most one group; combining `--staged` or `--working`
with `--base`/`--head` exits 2 with a specific error message:

| Command | What it diffs |
|---|---|
| `pushbadger analyze` | `<resolved-base>...HEAD` |
| `pushbadger analyze --staged` | staged changes (HEAD → index) |
| `pushbadger analyze --working` | unstaged changes (index → worktree) |
| `pushbadger analyze --base X` | `X...HEAD` |
| `pushbadger analyze --head Y` | `<resolved-base>...Y` |
| `pushbadger analyze --base X --head Y` | `X...Y` |

**Base branch resolution** (in order):
1. `--base` flag
2. `git symbolic-ref refs/remotes/origin/HEAD`
3. First of `main`, `master`, `trunk` that exists
4. Hard fail (exit 2) if none found

## How matching works

Each rule defines a named area, a priority, and a list of
[doublestar](https://github.com/bmatcuk/doublestar) glob patterns. On each run:

1. The changed file list is collected from `git diff`.
2. Every file path is normalized to lowercase before pattern matching.
3. Each file is tested against every rule's patterns in priority order.
4. A file may match multiple areas and will appear in each matched area.
5. Files that match no rule are collected into the `unclassified` area, which
   is always sorted last regardless of how many areas matched.
6. Within each area, files are sorted by path. Areas are sorted by priority
   (ascending), then by name for ties.

### Matching examples

Using the default embedded ruleset:

| File path | Matched area(s) | Notes |
|---|---|---|
| `services/payments/processor.go` | `payments` | matches `**/payment*/**` |
| `pkg/auth/session_test.go` | `auth`, `tests` | matches `**/auth/**` and `**/*_test.go` |
| `internal/retry/backoff.go` | `retry` | matches `**/backoff*` |
| `docs/adr/0001-risk-model.md` | `docs` | matches `**/*.md` and `**/docs/**` |
| `scripts/release.sh` | `unclassified` | no rule covers `.sh` files |

A file like `pkg/auth/session_test.go` appears in both `auth` (priority 20)
and `tests` (priority 70). Because areas are sorted by priority, `auth` always
precedes `tests` in the output.

## Example output

**Text:**
```
$ pushbadger analyze --base main

Risk analysis: main...HEAD

payments (1 file)
  internal/payments/checkout.go

auth (2 files)
  internal/auth/login.go
  internal/auth/session.go

db (1 file)
  db/migrations/0001_add_users.sql

unclassified (1 file)
  cmd/app/main.go

5 files, 4 areas
```

**JSON (`--format json`):**
```json
{
  "base": "main",
  "head": "HEAD",
  "mode": "diff",
  "ruleset_version": 1,
  "files": [
    { "path": "cmd/app/main.go" },
    { "path": "db/migrations/0001_add_users.sql" },
    { "path": "internal/auth/login.go" },
    { "path": "internal/auth/session.go" },
    { "path": "internal/payments/checkout.go" }
  ],
  "areas": [
    {
      "name": "payments",
      "priority": 10,
      "files": [{ "path": "internal/payments/checkout.go" }]
    },
    {
      "name": "auth",
      "priority": 20,
      "files": [
        { "path": "internal/auth/login.go" },
        { "path": "internal/auth/session.go" }
      ]
    },
    {
      "name": "db",
      "priority": 30,
      "files": [{ "path": "db/migrations/0001_add_users.sql" }]
    },
    {
      "name": "unclassified",
      "priority": 9223372036854775807,
      "files": [{ "path": "cmd/app/main.go" }]
    }
  ]
}
```

**Staged changes:**
```
$ pushbadger analyze --staged

Risk analysis: staged changes (HEAD → index)

auth (1 file)
  internal/auth/newfeature.go

1 file, 1 area
```

## Version

`--version` reports both the app version and the active ruleset version:

```
$ pushbadger --version
pushbadger v0.1.0-alpha (ruleset version 1)
```

The `ruleset_version` field is also included in every JSON report, so consumers
can detect when reports are produced with different rulesets.

## Ruleset format

The default embedded ruleset covers eight areas. Override at runtime with
`--rules path/to/rules.yaml`:

```yaml
version: 1
rules:
  - area: payments
    priority: 10
    patterns:
      - "**/payment*/**"
      - "**/billing/**"

  - area: auth
    priority: 20
    patterns:
      - "**/auth/**"
      - "**/session*"
```

**Schema:**
- `version` — integer; propagated to `ruleset_version` in JSON reports and `--version` output
- `rules[].area` — name of the risk area
- `rules[].priority` — lower number = higher priority; controls sort order in output
- `rules[].patterns` — [doublestar](https://github.com/bmatcuk/doublestar) glob patterns matched against the lowercased file path

## Limits and truncation

- Max **200 files** per report
- Max **200 KB** raw diff output
- If either limit is exceeded, `truncated: true` is set in the JSON report, a
  `truncation_reason` object is added naming which limit was hit, and a warning
  is written to stderr

```json
"truncated": true,
"truncation_reason": {
  "reason": "diff_size",
  "max_files": 200,
  "max_diff_size_kb": 200
}
```

`reason` is one of `"files"`, `"diff_size"`, or `"files_and_diff_size"`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 2 | Usage error or git error (not in a repo, bad flags, etc.) |
| 3 | Internal failure |

## Determinism

Same diff + same ruleset version = byte-identical output (text and JSON). No
timestamps are included in output.

**What is deterministic:**
- File lists within each area are sorted by path.
- Areas are sorted by priority (ascending), then name for ties.
- `unclassified` is always last.
- The `ruleset_version` field lets consumers detect ruleset changes between runs.

The integration test suite (`test/integration/`) creates a real temporary git
repository on every run and verifies these properties end-to-end:

- The same fixture diff produces byte-identical JSON across five consecutive runs.
- A file that matches multiple areas always appears in those areas in the same
  priority order across repeated runs.
- Files with no matching rule always land in `unclassified`, which is always last.
- Every JSON report includes a `ruleset_version` field.

Run the full suite:

```sh
go test ./...
# or
make test
```

Run only integration tests:

```sh
go test ./test/integration/
```

## Development

```sh
make build   # compile to ./pushbadger
make test    # go test ./...
make lint    # go vet ./...
make clean   # remove ./pushbadger binary
```

## Self-referential smoke test

Running PushBadger on its own repository is a good sanity check and doubles as
a canonical determinism fixture (same commit range + same ruleset = same output):

```
$ pushbadger analyze --base $(git rev-list --max-parents=0 HEAD) --head HEAD

Risk analysis: <initial-sha>...HEAD

config (2 files)
  config/embed.go
  config/risk_rules.yaml

tests (2 files)
  internal/analyzer/heuristics_test.go
  test/integration/analyze_test.go

docs (2 files)
  CHANGELOG.md
  README.md

unclassified (10 files)
  LICENSE
  cmd/pushbadger/main.go
  go.mod
  go.sum
  internal/analyzer/heuristics.go
  ...
```

(Exact file counts grow as the repo does; area assignments and sort order are stable.)

## Acknowledgments

Built with assistance from [Claude Code](https://claude.ai/claude-code),
Anthropic's AI coding assistant.

Copyright 2026 Adam Deane
