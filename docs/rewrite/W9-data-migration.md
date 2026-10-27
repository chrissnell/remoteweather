# W9 — Migrating the existing weather data

**Depends on:** W4 (the new schema must exist), W0 (spike findings).
**Produces:** `docs/data-migration.md` and the generated migration scripts.

## Why this is its own work item

W4 designs the new schema; nothing owns moving ten years of the owner's data into it. The owner's
constraint: *"I don't want to lose the data from my existing database."*

## Measured starting state

Source database is `weather_v2_0_0`.

| | Span | Size |
|---|---|---|
| raw `weather` hypertable | 2025-10-22 → now, **366 days** | **19 GB**, 53 chunks (7-day) |
| `weather_1d` continuous aggregate | 2025-01-01 → now, **660 days** | **3.5 MB** |
| `weather_1h` | 2 years retention | not measured — measure it |
| **rollup history with no raw behind it** | **~294 days** | — |

The retention policy on raw is 365 days and **is active** (confirmed via
`timescaledb_information.jobs`), so raw is a rolling one-year window. The `weather_1d` aggregate
reaches back to 2025-01-01, which means roughly **294 days of daily and hourly rollup cannot be
recomputed from anything** — the raw rows behind it were deleted long ago.

`weather-backup`/`weather-restore` do not help: they are `SELECT * FROM weather` and
`INSERT INTO weather`, covering only the raw hypertable, never the aggregates. They were also one-time
tools for a previous migration and are retired (see the capability inventory).

## Scope

### 1. Decide, explicitly, what history is carried

Two options; the owner picks, and this document must present them as a decision rather than assume:

- **Raw only.** The new database starts with ~366 days and regenerated aggregates cover the same
  window. Clean and simple; the ~294 days of older daily rollup is lost.
- **Raw plus the rollup archive.** The 294-day remainder is imported as **pre-seeded rollup rows**,
  because there is no raw data to recompute it from, with a union at the boundary for reads.

Cost of preserving it is trivial — the entire daily table is 3.5 MB — so the recommendation is to take
a copy regardless, before anything else happens:

```sql
CREATE TABLE weather_1d_archive AS SELECT * FROM weather_1d;
CREATE TABLE weather_1h_archive AS SELECT * FROM weather_1h;
```

That makes the decision reversible. Note that `pg_dump -t weather_1d` captures only a *view
definition* — the rows live in an internal `_timescaledb_internal._materialized_hypertable_N` — which
is why copying through the view first is required. Copying through the view is also correct because
the aggregates are declared `materialized_only = false`, so the view unions materialised and
real-time buckets.

### 2. The transform

Readings move from the old 143-column `weather` table to the new generated schema. Specify the
column-by-column mapping, driven by the metric registry (W2) so it is generated rather than
hand-written, and state what happens to old columns with no registry entry.

### 3. Order of operations

This sequence is not negotiable and the document must state why:

1. Export/copy, and verify the copy, **before** touching anything.
2. Create the new schema with **no continuous aggregates yet** — loading into a hypertable that
   already has aggregates defined is slower and accumulates invalidation records.
3. Load and transform.
4. Create the aggregates.
5. **Force a full materialisation explicitly** with `CALL refresh_continuous_aggregate(cagg, NULL,
   NULL)`. A refresh *policy* will not backfill outside its `start_offset`, so a bulk historical load
   otherwise materialises only recent data — and it fails silently, looking correct until someone asks
   for last winter.
6. Add policies, then compression, then retention.
7. Reconcile before decommissioning anything.

### 4. A hazard to document prominently

**Do not run a manual refresh over a historical range on the old database.** Retention dropping raw
chunks deliberately does *not* invalidate already-materialised aggregate buckets — that is why
`weather_1d` holds 660 days on 366 days of raw. But an explicit
`refresh_continuous_aggregate` over a window where the raw data is gone can overwrite those buckets
with nothing. Behaviour has varied across TimescaleDB versions; treat it as live.

### 5. Reconciliation

Row counts per metric per year, old versus new, plus spot checks on known values — record highs and
lows for specific remembered days. The old database stays read-only until this passes.

### 6. Validate by running both

The strongest available check: run the Go and Rust systems against the same inputs for a period, with
the Rust writes going to the new database, and diff the results. This is the validation story for
everything that has no test oracle, and it is only possible because there is a single operator.

## Do not

- Design the target schema (W4).
- Decide the history question unilaterally — present it with a recommendation.
- Decommission or modify the source database. It is read-only for the duration.
- Fabricate raw rows from aggregate means to "recreate" history. Averages of fabricated averages are
  not the original numbers, and min/max would be destroyed outright.

## Done when
- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`.
- The history decision is presented with a recommendation and a cost for each option.
- The archive copy step is specified and verified.
- The ordered procedure is written with the explicit-refresh step called out.
- Reconciliation queries are written and runnable.
