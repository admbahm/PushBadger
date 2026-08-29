# Development Guide

## Local setup

Requirements:

- Go 1.24+
- `git` on `PATH`

Common commands:

```sh
make build
make test
make lint
make clean
```

Equivalent direct commands:

```sh
go build -o pushbadger ./cmd/pushbadger
go test ./...
go vet ./...
```

Integration tests:

```sh
go test ./test/integration/
```

## Working style

Prefer small, reviewable branches with one architectural purpose. Do not combine unrelated cleanup with trust-boundary or behavior changes unless the cleanup is required to make the change safe.

During implementation, run focused tests for the area being changed. Before handoff, run the full suite and record the exact commands and outcomes.

## Change requirements

For behavior changes:

- state the observable behavior being changed;
- define constraints and non-goals;
- add or update tests;
- preserve deterministic ordering and output guarantees;
- distinguish pre-existing failures from failures introduced by the branch.

For parser, schema, policy, or admission changes:

- treat the input as untrusted;
- add adversarial tests, not only happy-path tests;
- test neighboring records and masking behavior;
- test malformed and boundary representations;
- attempt to disprove the intended admission rule independently of the implementation.

## Review standard

A useful finding should answer:

- What is the claim?
- What behavior or code path is affected?
- What evidence supports it?
- Can it be reproduced?
- What was expected and observed?
- What attempt was made to disprove it?
- Is it introduced by this change?

Only findings with sufficient evidence should block a change. Style and plausible-but-unproven concerns should remain non-actionable.

## Pull requests

PRs should be narrowly scoped and include:

- objective;
- architecture or behavioral impact;
- explicit non-goals;
- tests added or changed;
- commands run and outcomes;
- known baseline failures, if any;
- acceptance evidence.

Do not claim success from a green subset while hiding a relevant failing suite.

## Determinism checklist

Before merging changes to deterministic output or policy code, check for:

- timestamps or clock-derived values;
- unstable map iteration;
- environment-specific paths or values;
- randomized ordering;
- implicit network access;
- runtime-dependent resource ceilings that are not explicit policy;
- numeric coercion through floating point;
- parser behavior that changes submitted meaning.

## Current milestone

The current milestone is the Evidence Contract v1 port described in `docs/evidence-contract-port.md`.

Keep the first implementation boring: deterministic validation only. Agent/provider orchestration comes later.
