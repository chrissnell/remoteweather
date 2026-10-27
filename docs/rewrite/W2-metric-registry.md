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

## Open question for the owner

The registry's authoring format: TOML, RON, or Rust consts compiled directly. TOML reads best and is
editable by non-Rust tooling; Rust consts get compile-time validation for free. Worth deciding
before the prompt is written.

## Output contract

This document must end with the four required sections from `docs/rewrite/README.md`: **Decisions**
(numbered one-liners), **Open questions** (with recommendations), **Assumptions about other work
items**, and **Proposed amendments to `DESIGN.md`**. W6 consumes these.
