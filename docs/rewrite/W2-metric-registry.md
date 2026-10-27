# W2 — Metric registry and codegen spine

**Status: OUTLINE, not ready to run.** Scope is settled; the prompt still needs writing.

**Depends on:** W0. Runs in parallel with W1 and W3.
**Produces:** the registry format, a `cargo xtask codegen` implementation, and committed generated
output plus a CI drift check.

## Scope

The single source of truth from which everything else is generated. Per the standing constraints,
one registry entry carries at minimum: key, label, unit, aggregation strategy, tier (core/extended),
DB column name, protobuf field number, and the version it was introduced in.

From it, generate: DDL and migration bodies, the four CAGG definitions (template × registry, with
the aggregation strategy selecting the SQL function), Rust `Metric`/`Reading` types and accessors,
`reading.proto`, the OpenAPI schema, and TypeScript types plus the label/unit table the UI renders
from.

Two disciplines the prompt must enforce:
- identifiers are **append-only and never reused**; removal marks deprecated
- generated output is **committed**, and `xtask codegen --check` fails CI on drift

Also in scope: the migration *generator* — registry diffed against a committed schema snapshot,
emitting a timestamped migration. Changes are classified, and unsafe ones (aggregation strategy
change, column rename, type widening) are **refused by default** rather than silently rendered as
destructive SQL. Identifier reuse is a fatal codegen error.

## Why this isn't written yet

It is deliberately downstream of W0's spike. Two findings would change it materially:

1. whether `ALTER TABLE ADD COLUMN` works on compressed chunks without decompressing — this decides
   whether "new metric" is genuinely a safe auto-emitted migration or needs an operator step
2. whether hierarchical CAGGs work on the installed version, and whether `sum`/`count` rollups are
   numerically identical to aggregating raw — this decides what the CAGG template actually emits

Write this prompt after reading `docs/spike-findings.md`.

## Decided: the registry is authored in TOML

Owner decision — "whatever is cleanest and easiest" — resolved to **TOML**, for these reasons:

- it is human-editable declarative data, which is what a metric list is
- parsing is trivial with `serde`, and the file is readable by non-Rust tooling
- no recompile is needed to inspect or diff the registry
- the compile-time-validation argument for Rust consts does not actually buy the safety that matters
  here: the critical invariants are **append-only identifiers and no reuse of column names or
  protobuf field numbers**, and those must be enforced by the generator against the committed schema
  snapshot regardless of authoring format. A `const` cannot catch a reused field number either.

So: TOML as the source of truth, and the generator owns invariant enforcement.

## Output contract

This document must end with the four required sections from `docs/rewrite/README.md`: **Decisions**
(numbered one-liners), **Open questions** (with recommendations), **Assumptions about other work
items**, and **Proposed amendments to `DESIGN.md`**. W6 consumes these.
