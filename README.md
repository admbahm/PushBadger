# PushBadger

PushBadger analyzes git diffs and maps changed files to risk areas using
deterministic, path-based heuristics. No AI — just fast, reproducible signal
about which parts of your codebase are touched by a change.

## Install

```sh
go install pushbadger/cmd/pushbadger@latest
```

Or build from source:

```sh
git clone <repo>
cd pushbadger
go build -o pushbadger ./cmd/pushbadger
```

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

**Diff modes** — pick at most one group; combining --staged or --working with
--base/--head exits 2 with a specific error message:

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

## Self-referential example

Running pushbadger on the pushbadger repo itself makes a good smoke test and
doubles as a canonical determinism fixture (same ruleset, same diff, always the
same output).

```
$ pushbadger analyze --base $(git rev-list --max-parents=0 HEAD) --head HEAD

Risk analysis: <initial-sha>...HEAD

config (6 files)
  CHANGELOG.md [→ matched by docs too]
  config/embed.go
  config/risk_rules.yaml
  go.mod
  go.sum
  ...

tests (2 files)
  internal/analyzer/heuristics_test.go
  test/integration/analyze_test.go

docs (3 files)
  CHANGELOG.md
  README.md
  ...

unclassified (N files)
  cmd/pushbadger/main.go
  internal/analyzer/heuristics.go
  ...

N files, M areas
```

(Exact output varies with the commit range; the structure is stable.)

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
can detect if they are comparing reports produced with different rulesets.

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

**Matching rules:**
- Patterns use `github.com/bmatcuk/doublestar/v4` (`**` matches path separators)
- Matching is case-insensitive (paths are lowercased before matching)
- A file can match multiple areas and will appear in each
- Files with no match appear in the `unclassified` area at the end

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

Same diff + same ruleset = byte-identical output (text and JSON). No timestamps
are included in output. The integration test suite asserts this property on
every run.

Copyright 2026 Adam Deane
