# Changelog

## v0.1.0-alpha

### What's in

- `pushbadger analyze` command with human-readable text output and `--format json`
- Git diff resolution: `<base>...HEAD`, `--staged`, `--working`, `--base X --head Y`
- Base branch auto-detection: `--base` flag → `origin/HEAD` → `main`/`master`/`trunk`
- Path-based risk area mapping via YAML ruleset (doublestar glob matching, case-insensitive)
- Embedded default ruleset covering: payments, auth, db, retry, config, infra, tests, docs
- Custom ruleset override via `--rules`
- Renamed files treated as modified (new path used); deleted and binary files tagged
- 200-file / 200 KB diff cap with `truncated` flag and stderr warning
- Deterministic output: same diff + same ruleset = byte-identical text and JSON
- Exit codes: 0 success, 2 usage/git error, 3 internal failure
- Table-driven unit tests for the rules matcher including a determinism assertion
- Integration tests against a fixture git repo (staged diff, end-to-end determinism)

### Deferred (not in this release)

- AI-powered content analysis
- CI integration / PR annotations
- Caching of diff or rule results
- Retry logic / resilience patterns
- Remote ruleset loading
- Watch mode / `--watch` flag
- Configuration file (`.pushbadger.yaml`)
- Severity levels on areas
- `pushbadger explain` or per-file detail commands
