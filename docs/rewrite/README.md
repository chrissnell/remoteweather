# RemoteWeather Rust rewrite — work item prompts

Agent prompts for the Rust rewrite, one file per work item. These are **inputs to agents**, not
documentation of a finished design. Edit them freely; they are meant to be iterated on before
anything runs.

## How these fit together

```
W0  spike → decisions          (serial; everything depends on it)
     │
     ├── W1  frozen contract    (iOS + website)      ─┐
     ├── W2  metric registry + codegen               ─┤ parallel
     └── W3  service architecture design             ─┘
              │                    │
              │                    └── W4  database schema  (needs W2)
              └── W5  management UI design  (needs W3 + W2)
```

| Item | File | Produces | After |
|---|---|---|---|
| W0 | `W0-spike-and-decisions.md` | `docs/spike-findings.md`, `docs/DESIGN.md` | — |
| W1 | `W1-frozen-contract.md` | `docs/legacy-api-contract.md`, `contract/` fixtures + conformance test | W0 |
| W2 | `W2-metric-registry.md` | registry format + `xtask codegen` | W0 |
| W3 | `W3-service-architecture.md` | `docs/service-architecture.md` | W0 |
| W4 | `W4-database-schema.md` | migrations, CAGG generation | W2, W0 |
| W5 | `W5-management-ui.md` | `docs/management-ui-design.md` | W3, W2 |

Implementation work items follow once these land. The design/implement split is **deliberate** for
the pipeline (W3 → runtime spine) and the UI (W5 → UI build); it is not an oversight.

## Document ownership — read this before editing any prompt

The most likely way this set degrades is two prompts claiming the same ground. The boundaries:

| Document | Owns | Must not contain |
|---|---|---|
| `DESIGN.md` (W0) | **What** — decisions and constraints, with rationale | Trait definitions, signatures, buffer sizes, module layout |
| `service-architecture.md` (W3) | **How** — signatures, module boundaries, policies, buffer sizes | New decisions that contradict `DESIGN.md` without an amendment |
| `legacy-api-contract.md` (W1) | The frozen external surface, as it actually behaves today | Any proposed improvement to that surface |
| `management-ui-design.md` (W5) | Operator UI only | Anything about the public weather website |

## `DESIGN.md` amendment protocol

`DESIGN.md` is the shared context every later work item reads. It will be wrong about something.

- Decisions are **not** edited in place.
- Any work item that finds a decision wrong adds an entry to the `## Amendments` section:
  date, which decision, what was found, what replaces it.
- The original decision stays, marked superseded. Future readers need to know what was tried.

## Prompt conventions

Every prompt assumes a **fresh agent with no conversation history**. Hence:

1. It starts by reading `DESIGN.md` rather than restating decisions.
2. It follows **design → stop for review → implement**. Design work items stop at a document.
3. It has a **Do not** section. This is load-bearing — it's what keeps agents inside their lane.
4. Where it references the existing Go implementation, it says whether that's a reference for
   *behaviour* (copy it) or a *cautionary tale* (don't copy it). The Go repo is read-only.

## Standing constraints

Carried forward from the design discussion; `DESIGN.md` will record these with full rationale.

- **Rust + tokio.** Greenfield; the Go implementation stays deployable throughout.
- **The legacy REST contract is frozen.** Both the iOS app and the existing weather website keep
  working. New endpoints are additive. See W1 for the exact surface.
- **The public weather website is carried forward as-is.** Not redesigned. W3 owns how it's served.
- **Storage stays a wide table**, not narrow/EAV. Postgres stores a NULL as one bit, so wide costs
  ~`42 + 4×populated` bytes per row against narrow's ~44 bytes *per metric*. Narrow measured 7–9×
  worse on density with no crossover.
- **Raw data is retained indefinitely with compression enabled.** 19 GB/year uncompressed; ~16 GB
  for ten years at 12×. This makes continuous aggregates disposable, which is what makes adding a
  metric cheap.
- **One metric registry generates everything** — DDL, migrations, CAGG SQL, Rust types, protobuf,
  OpenAPI, TypeScript. Identifiers are allocated once and never reused.
- **Schema changes only via migrations.** No DDL at startup.

## Measured facts

Numbers from the existing system, so prompts don't have to re-derive them.

| | |
|---|---|
| Raw `weather` hypertable | 2025-10-22 → now, 366 days, **19 GB**, 53 chunks (7-day) |
| `weather_1d` CAGG | 2025-01-01 → now, 660 days, **3.5 MB** |
| History only in CAGGs (no raw behind it) | ~294 days |
| Retention | raw 365d · 1m 1mo · 5m 6mo · 1h 2y · 1d 10y |
| Compression configured | **none** |
| Go `Reading` struct | 217 fields · raw table ~143 columns · each CAGG view 397 columns |
| Aggregation calls across the 4 CAGG views | `avg` 540 · `max` 524 · `min` 504 · `sum` 12 · `circular_avg` 4 |
| Management API | 46 routes |
| Frontend | ~16.5k lines JS/CSS, no build step; `management.js` alone 2,819 lines |
| Test coverage | 13 test files; 0% on managers, storage, timescaledb, database, types, and 6 of 7 drivers |
| CI | release-on-tag only; no test, vet, fmt, or lint on PR |
