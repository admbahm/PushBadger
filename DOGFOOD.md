# Dogfood notes

Running PushBadger against its own repository. Observations captured here
feed ruleset decisions, documentation, and product direction.

---

## 2026-04-22 — v0.2.0, initial commit range

Command:
```
pushbadger analyze --base $(git rev-list --max-parents=0 HEAD) --head HEAD
```

Output (20 files, 5 areas):
```
config (3 files)
  .github/workflows/ci.yml     ← also in infra
  config/embed.go
  config/risk_rules.yaml

infra (2 files)
  .github/REPO_META.md         ← also in docs
  .github/workflows/ci.yml     ← also in config

tests (2 files)
  internal/analyzer/heuristics_test.go
  test/integration/analyze_test.go

docs (4 files)
  .github/REPO_META.md         ← also in infra
  CHANGELOG.md
  CONTRIBUTING.md
  README.md

unclassified (11 files)
  LICENSE
  Makefile
  cmd/pushbadger/main.go
  go.mod / go.sum
  internal/analyzer/heuristics.go
  internal/analyzer/rules.go
  internal/git/diff.go
  internal/git/repo.go
  internal/output/formatter.go
  pkg/types/risk.go
```

### Observations

**Multi-match working as designed**
- `ci.yml` hits both `config` (`**/*.yaml`) and `infra` (`**/.github/**`). Accurate — it's infra-as-config. Both matches are meaningful.
- `REPO_META.md` hits both `infra` (`**/.github/**`) and `docs` (`**/*.md`). Reasonable; a `.github/` markdown file is legitimately both.

**Unclassified gaps worth noting**
- `Makefile` — no rule covers bare `Makefile`. Low priority; build tooling is a thin category.
- `go.mod` / `go.sum` — config rule covers `*.yaml`, `*.toml`, `*.env` but not `*.mod` / `*.sum`. Dependency manifests are arguably config. Worth revisiting when the ruleset gets a version bump.
- `LICENSE` — extension-less file, no rule matches. Not worth adding a rule for.
- All core source files (`internal/`, `pkg/`, `cmd/`) land in unclassified. Expected — the default ruleset targets application-level risk areas (payments, auth, db), not infrastructure of the tool itself. The self-analysis is a structural test, not a meaningful risk signal.

**No surprises in test/docs/config classification.** The areas that matter for this repo (config files, test files, docs) are correctly classified.
