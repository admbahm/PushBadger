# Evidence Contract v1 parity corpus

This directory defines the behavioral parity target for the Go implementation of `pushbadger validate`.

The authoritative contract is the accepted DevInsight Evidence Contract v1 merged from PR #15. PushBadger should preserve the contract's externally observable admission behavior without copying Python runtime behavior blindly.

## Required parity

The Go implementation must preserve these guarantees:

- JSON Schema draft 2020-12 validation using `.review/review-evidence.schema.json` as the structural authority.
- Strict rejection of duplicate object keys, including identical duplicates, escaped-equivalent keys, and nested duplicates.
- Rejection of NaN and infinities before admission.
- Exact numeric meaning for supported JSON number representations; do not route evidence numbers through binary floating point.
- Integer semantics must accept mathematically integral decimal/exponent representations where the schema permits an integer and reject fractional near-integers.
- Large integer tokens must reach schema validation without an arbitrary lexical digit ceiling introduced by the parser.
- `code_path.qualification` is required only for `code_path` evidence and is one of `OBSERVED`, `STRONG`, or `UNAVOIDABLE`.
- `HIGH_CONFIDENCE` requires `STRONG` or `UNAVOIDABLE` code-path evidence.
- `PROVEN` requires completed executable evidence or an `UNAVOIDABLE` code path. Unexecuted executable evidence must not be masked by another admissible evidence record.
- Verification IDs and finding IDs must be unique.
- PASS requires at least one required verification and every required verification must PASS; actionable findings are forbidden.
- FAIL requires at least one admissible actionable finding.
- INCONCLUSIVE requires incomplete required verification (`NOT_RUN` or `UNKNOWN`) and cannot contain actionable findings.
- Optional verification must not determine the verdict.
- Invalid neighboring records must not be masked by valid findings or verification records.
- Persisted evidence validation must fail the batch when any discovered artifact is invalid.

## Accepted reference case

`.review/examples/benchmark-001.json` is a known-valid FAIL document from the DevInsight benchmark. A conforming PushBadger implementation must admit it.

## Adversarial history to preserve

The original validator was hardened through adversarial review. Two parser-boundary defects are especially important for the Go port:

1. A near-integer JSON number such as `0.99999999999999999` must not be rounded to `1` and then admitted as an integer.
2. Large integer tokens must not be rejected solely because the host runtime has an unrelated decimal-string-to-integer digit limit when the contract itself places no such maximum.

These are portability lessons, not Python-specific implementation requirements. The desired invariant is:

> Exact semantic preservation where the implementation claims support. Explicit failure at resource boundaries is acceptable. Silent coercion, reinterpretation, or incorrect admission is not.

## First implementation milestone

The first implementation milestone is intentionally narrow:

```text
pushbadger validate <evidence.json>
```

No model invocation, network access, PR comments, approval/merge behavior, or investigator orchestration belongs in this milestone. The validator is part of PushBadger's deterministic trusted core.
