# Architecture

## Purpose

PushBadger provides deterministic assurance around code changes. It currently maps git diffs to risk areas using path-based rules. The next stage extends that model with deterministic admission of review evidence while keeping nondeterministic investigators outside the trusted core.

## Core model

```text
MAP -> INVESTIGATE -> ADMIT
```

### MAP

PushBadger's existing deterministic analysis layer:

- reads a bounded git diff;
- normalizes changed paths;
- applies a versioned ruleset;
- maps files into risk areas;
- emits stable, ordered output.

A risk classification identifies where investigation should focus. It does not prove a defect.

### INVESTIGATE

An external actor examines the change. It may be:

- a human reviewer;
- Codex;
- Gemini;
- a local model;
- another review system.

Investigators are explicitly untrusted. They may be nondeterministic, mistaken, incomplete, or unavailable. Their job is to produce machine-readable evidence, not to decide what PushBadger must believe.

### ADMIT

The deterministic trust boundary. Submitted evidence is processed through:

```text
raw evidence
  -> strict parsing
  -> structural/schema validation
  -> semantic policy validation
  -> finding admission
  -> verdict admission
```

Only admitted evidence may justify an actionable finding or final verdict.

## Trust boundary

The system assumes everything before ADMIT can be wrong.

The admission layer must therefore defend against malformed input, duplicate keys, numeric reinterpretation, invalid confidence escalation, record masking, contradictory verdict state, and other representations that could cause the software to accept meaning the submitter did not actually provide.

The invariant is:

> Exact semantic preservation where the implementation claims support. Explicit failure at resource boundaries is acceptable. Silent coercion, reinterpretation, or incorrect admission is not.

## Determinism

PushBadger's existing determinism contract remains intact:

- no timestamps in deterministic output;
- stable ordering;
- versioned rulesets/contracts;
- no dependency on model output for trusted decisions;
- no required network access;
- explicit bounded inputs where necessary.

Future orchestration may invoke nondeterministic external investigators, but their output must be treated as untrusted input to the deterministic core. Orchestration must not redefine the semantics of MAP or ADMIT.

## Product boundary

The intended product architecture is:

```text
                 UNTRUSTED / NONDETERMINISTIC

       Codex      Gemini      Local      Human
          \          |          |          /
                   evidence
                      |
================ TRUST BOUNDARY ================
                      |
                 PushBadger ADMIT
                      |
             admitted findings/verdict

                 TRUSTED / DETERMINISTIC
```

PushBadger may eventually offer convenience commands that launch investigators, but the trusted decision path must remain independently testable without AI or network access.

## Near-term evolution

1. Preserve the existing `analyze` behavior.
2. Port Evidence Contract v1 into Go.
3. Add `pushbadger validate <evidence.json>`.
4. Prove behavioral parity against the accepted reference corpus.
5. Use PushBadger's risk map as deterministic context for external investigators.
6. Add orchestration only after validation is stable in real use.

## Non-goals for the current phase

- autonomous merge or approval;
- generic coding-agent behavior;
- hosted SaaS architecture;
- agent memory;
- multi-agent swarms;
- semantic verification of arbitrary prose;
- replacing CI, SAST, or conventional test suites;
- weakening deterministic guarantees to accommodate a provider.
