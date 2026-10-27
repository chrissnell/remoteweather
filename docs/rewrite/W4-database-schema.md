# W4 — Database schema, migrations, and continuous aggregates

**Status: OUTLINE, not ready to run.** Scope is settled; the prompt still needs writing.

**Depends on:** W2 (generates the DDL) and W0 (spike results). Runs in parallel with W3.
**Produces:** the migration set, CAGG generation, compression and retention configuration, and
CI-verified application against a fresh TimescaleDB.

## Scope

- `observations` — wide hypertable, columns generated from the registry
- `stations` dimension table with a surrogate `station_id int`. Not `stationname text` plus
  `stationtype text` repeated on every row, which is what the Go schema does: ~20 bytes instead of 4
  on every row of a hundred-million-row table, and it weakens compression `segmentby`
- `station_metrics` — which metrics each station actually emits, so the API can describe a station's
  capabilities without probing data. The Go code has a TODO for exactly this at
  `internal/weatherstations/grpcreceiver/station.go:64`
- compression from day one: `segmentby = station_id`, `orderby = time DESC`, policy at 7 days
- **no raw retention policy** — raw is permanent, which is what makes aggregates disposable
- hierarchical CAGGs storing composable state (`sum`/`count` or `stats_agg`), never a finished `avg`
- an `extra jsonb` escape hatch for unmodelled vendor fields: sparse, never aggregated
- `sqlx::migrate!()` only; no DDL at startup

## Why this isn't written yet

Directly gated on W0's spike. If `ADD COLUMN` on compressed chunks fails, or hierarchical CAGGs
aren't available, the schema strategy changes rather than just the migration mechanics. Writing this
before the measurements exist means writing it twice.

## Related but separate

Migrating the existing `weather_v2_0_0` data is its own work item, not this one. Its known shape:
import raw (366 days, 19 GB), and decide whether to carry the ~294 days of `weather_1d`/`weather_1h`
rollup that has no raw behind it — that portion cannot be recomputed and arrives as pre-seeded
rollup rows if it's wanted at all. Reconciliation by row count and spot values before the old
database is decommissioned.

## Output contract

This document must end with the four required sections from `docs/rewrite/README.md`: **Decisions**
(numbered one-liners), **Open questions** (with recommendations), **Assumptions about other work
items**, and **Proposed amendments to `DESIGN.md`**. W6 consumes these.
