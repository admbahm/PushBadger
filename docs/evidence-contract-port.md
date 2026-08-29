# Evidence Contract v1 Port Plan

## Reference

The accepted Evidence Contract v1 reference implementation lives in DevInsight. Its acceptance checkpoint was merged to `main` at:

`91b9a38e8b7360fab0f8a29274d30b048066bacd`

The accepted reference includes strict parsing, schema validation, semantic admission policy, persisted-artifact validation, and adversarial tests. At acceptance, the evidence suite contained 58 passing tests, Rust tests were 21 passing, and an independent 11,400-case policy cross-product produced zero mismatches.

PushBadger should port the behavior, not redesign it during the port.

## Goal

Implement a deterministic Go validator exposed initially as:

```sh
pushbadger validate <evidence.json>
```

For every supported reference input, PushBadger must make the same admission decision as the accepted DevInsight implementation.

## Porting invariants

- Preserve the v1 finding classifications and verdict semantics exactly.
- Preserve strict rejection of duplicate object keys, including nested and escaped-equivalent duplicates.
- Reject non-finite JSON numbers.
- Preserve represented numeric meaning through schema evaluation; do not route evidence numbers through `float64`.
- Ensure booleans cannot satisfy integer schema requirements.
- Preserve actionable-evidence requirements for `PROVEN` and `HIGH_CONFIDENCE`.
- Preserve required vs. optional verification behavior.
- Preserve duplicate finding/verification ID rejection.
- Preserve record-neighbor isolation: valid records must not mask invalid records.
- Preserve persisted-artifact batch semantics.
- Fail closed at explicit implementation resource boundaries.

## Go parser warning

The standard `encoding/json` decoder is not sufficient by itself for this trust boundary.

In particular:

- ordinary unmarshalling accepts duplicate object keys rather than rejecting them;
- decoding numeric values into `interface{}` normally produces `float64`, which can silently lose numeric meaning.

The implementation must therefore use a token-aware strict parsing strategy and exact numeric handling. `json.Decoder.UseNumber()` may be part of the solution, but it is not by itself proof that duplicate-key and schema semantics are correct.

Treat parser behavior as security-sensitive product behavior and test it independently.

## Required parity corpus

Before declaring the Go port complete, reproduce the accepted reference categories:

- canonical valid benchmark;
- schema-invalid documents;
- invalid finding/verdict combinations;
- `OBSERVED`, `STRONG`, and `UNAVOIDABLE` qualification cases;
- executable evidence completed/not completed cases;
- required and optional verification states;
- duplicate IDs;
- invalid neighboring records;
- duplicate JSON keys at multiple depths;
- escaped-equivalent duplicate keys;
- NaN and infinities;
- ordinary integers and zero/negative values;
- fractions and near-integers;
- integral and non-integral exponent notation;
- very large integer tokens, including values beyond common floating-point precision;
- large negative integers and clean schema error reporting;
- persisted artifact discovery and batch failure behavior.

Where practical, keep fixtures byte-identical to the reference corpus.

## Acceptance criteria

The port is accepted only when all of the following are true:

1. `pushbadger analyze` behavior is unchanged.
2. `pushbadger validate` requires no AI or network access.
3. Reference valid artifacts are admitted.
4. Reference invalid artifacts are rejected for the same semantic reason.
5. No tested input is silently reinterpreted before admission.
6. Duplicate keys and non-finite values are rejected before policy evaluation.
7. The adversarial policy matrix produces zero mismatches against an independently derived oracle.
8. `go test ./...` and `go vet ./...` pass.
9. Repeated validation of identical supported input is deterministic.
10. No agent orchestration or provider-specific code is introduced in this milestone.

## Explicit non-goals

This port does not need to support mathematically unbounded numeric representations, infinite document size, arbitrary prose truth verification, or every theoretical JSON resource extreme.

Finite limits are acceptable when they are explicit and fail closed. Accidental limits that silently change meaning or cause incorrect admission are not.

## After parity

Once the Go implementation proves parity, PushBadger becomes the canonical product implementation of evidence admission. DevInsight remains useful as historical/reference evidence during the transition, but duplicated validator logic should not evolve independently in both repositories.
