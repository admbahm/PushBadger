# AGENTS.md

## Mission

PushBadger is a deterministic PR assurance engine. Its trusted core maps changed files to risk areas and admits only evidence that satisfies explicit, machine-checkable contracts.

External AI tools may investigate changes and produce evidence, but PushBadger does not trust model output by default.

> Trust the evidence, not the agent.

## Engineering doctrine

1. Understand the system.
2. Discover its limits.
3. Build what ought to exist next.
4. Prove that it works.

Be brief, be succinct, be gone.

## Architectural invariants

- Determinism is a product requirement. Identical supported inputs must produce identical outputs.
- The trusted PushBadger core must not require AI or network access.
- Investigators are untrusted and nondeterministic evidence producers.
- Evidence crosses a trust boundary before it can affect an admission or verdict.
- Parsing and serialization are part of the trust boundary. Do not silently coerce submitted meaning.
- Risk heuristics scope investigation; they are not proof that a defect exists.
- Baseline failures are not new defects unless the proposed change introduces or worsens them.
- Prefer explicit resource limits over accidental runtime limits.
- No autonomous merge, approval, push, test weakening, or unrelated repository modification.

## Target architecture

```text
Git diff
   |
   v
MAP -- deterministic PushBadger risk analysis
   |
   v
INVESTIGATE -- external/untrusted human or agent
   |
   v
machine-readable evidence
   |
   v
---------------- TRUST BOUNDARY ----------------
   |
   v
ADMIT -- strict parse -> schema -> semantic policy -> verdict
```

The trusted product boundary is MAP + ADMIT. INVESTIGATE may be supplied by Codex, Gemini, a local model, another tool, or a human.

## Current development priority

Port the accepted Evidence Contract v1 reference implementation from DevInsight into PushBadger without changing its semantics.

The first product surface is intentionally small:

```sh
pushbadger validate <evidence.json>
```

Do not add agent orchestration, provider SDKs, GitHub commenting, merge automation, or network-dependent behavior until deterministic validation parity is proven.

## Finding model

Only these confidence classes are recognized by the review architecture:

- `PROVEN` -- reproduced by executable evidence or established by an unavoidable code path.
- `HIGH_CONFIDENCE` -- supported by strong code-path evidence.
- `POSSIBLE` -- plausible but insufficiently supported; non-actionable.
- `STYLE` -- non-functional; non-actionable.

Only `PROVEN` and `HIGH_CONFIDENCE` findings are actionable.

A valid actionable finding must identify the claim, affected behavior/path, evidence, expected vs. observed behavior when applicable, severity/confidence, and attempted disproof.

## Review behavior

When reviewing a change:

1. Read the complete diff and relevant surrounding code.
2. Identify the behavior actually changed.
3. Form concrete hypotheses.
4. Seek evidence that could prove or disprove each hypothesis.
5. Reproduce when practical.
6. Attempt to falsify every proposed actionable finding.
7. Separate pre-existing failures from change-owned failures.
8. Report only findings that survive falsification.

Do not convert uncertainty into severity.

## Go development commands

```sh
make build
make test
make lint

go test ./...
go test ./test/integration/
go vet ./...
```

Before declaring work complete, run the smallest relevant tests during development and the full required suite before handoff.

## Change discipline

- Keep changes narrowly scoped.
- Preserve existing CLI behavior unless the task explicitly changes it.
- Add tests for behavior changes and trust-boundary fixes.
- Prefer table-driven tests for policy matrices and parser edge cases.
- Avoid hidden nondeterminism: timestamps, random iteration order, environment-derived output, unstable map ordering, or implicit network state.
- Do not weaken tests to make a change pass.
- Do not silently broaden the Evidence Contract while porting it.

## Definition of done

A change is done when its intended behavior is explicit, constraints and non-goals are clear, acceptance criteria are binary, tests prove the behavior, and the implementation preserves PushBadger's determinism and trust-boundary guarantees.
