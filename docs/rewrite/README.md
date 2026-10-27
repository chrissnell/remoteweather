# RemoteWeather Rust rewrite — work item prompts

Agent prompts for the Rust rewrite, one file per work item. These are **inputs to agents**, not
documentation of a finished design. Edit them freely; they are meant to be iterated on before
anything runs.

## How these fit together

```
W0  spike → decisions              (serial; everything depends on it)
     │
     ├── W1   frozen contract       (iOS + website)   ─┐
     ├── W2   metric registry + codegen               ─┤ parallel
     └── W2a  component + capability model            ─┘
                │
                └── W3  service architecture  (needs W2a)
                        │              │
                        │              └── W4  database schema  (needs W2)
                        └── W5  management UI design  (needs W3 + W2)

     ├── W7   collector            (needs W2a + W3 + W2)
     └── W8   CI + release          (parallel with everything; start early)
                │
                └── W9  data migration  (needs W4)

W6  reconciliation + implementation plan   (needs ALL of the above)
```

| Item | File | Produces | After |
|---|---|---|---|
| W0 | `W0-spike-and-decisions.md` | `docs/spike-findings.md`, `docs/DESIGN.md` | — |
| W1 | `W1-frozen-contract.md` | `docs/legacy-api-contract.md`, `contract/` fixtures + conformance test | W0 |
| W2 | `W2-metric-registry.md` | registry format + `xtask codegen` | W0 |
| W2a | `W2a-component-model.md` | `docs/component-model.md` | W0 |
| W3 | `W3-service-architecture.md` | `docs/service-architecture.md` | W0, **W2a** |
| W4 | `W4-database-schema.md` | migrations, CAGG generation | W2, W0 |
| W5 | `W5-management-ui.md` | `docs/management-ui-design.md` | W3, W2 |
| W7 | `W7-collector.md` | `docs/collector.md`, wire protocol, spool schema | W2a, W3, W2 |
| W8 | `W8-ci-and-release.md` | CI workflows, `docs/release.md` | W0 |
| W9 | `W9-data-migration.md` | `docs/data-migration.md`, migration scripts | W4, W0 |
| W6 | `W6-reconciliation.md` | `docs/reconciliation.md`, decision ledger, `docs/implementation-plan.md` | all |

Implementation work items follow once these land. The design/implement split is **deliberate** for
the pipeline (W3 → runtime spine) and the UI (W5 → UI build); it is not an oversight.

## Document ownership — read this before editing any prompt

The most likely way this set degrades is two prompts claiming the same ground. The boundaries:

| Document | Owns | Must not contain |
|---|---|---|
| `DESIGN.md` (W0) | **What** — decisions and constraints, with rationale | Trait definitions, signatures, buffer sizes, module layout |
| `service-architecture.md` (W3) | **How** — signatures, module boundaries, policies, buffer sizes | New decisions that contradict `DESIGN.md` without an amendment |
| `component-model.md` (W2a) | What a component **is** — trait, triggers, capabilities, lifecycle states | Pipeline internals, buffer sizes, module layout |
| `legacy-api-contract.md` (W1) | The frozen external surface, as it actually behaves today | Any proposed improvement to that surface |
| `management-ui-design.md` (W5) | Operator UI only | Anything about the public weather website |

## `DESIGN.md` amendment protocol

`DESIGN.md` is the shared context every later work item reads. It will be wrong about something.

- Decisions are **not** edited in place.
- Any work item that finds a decision wrong adds an entry to the `## Amendments` section:
  date, which decision, what was found, what replaces it.
- The original decision stays, marked superseded. Future readers need to know what was tried.

## Output contract — how results come back

Design work items do **not** report in chat. Each one commits its output to the repository, because
the prompts instruct every agent to read its predecessors' documents from the tree — which only
works if they are in the tree.

- One branch and one PR per work item: `design/<item>`, e.g. `design/w2a-component-model`.
- Output lands under `docs/` at the path named in the prompt.
- **Merge a work item before starting anything that depends on it.** Do not run W3 against an
  unmerged W2a.

### Every design document must end with these four sections

This is what makes the outputs mechanically comparable instead of six essays that have to be held in
one head at once.

1. **Decisions** — numbered one-liners, each independently quotable. This is the section the owner
   reviews; it should be reviewable in ten minutes, with the prose read only where a decision looks
   wrong.
2. **Open questions** — anything the agent could not resolve, stated as a decision for the owner with
   options and a recommendation. Not a list of worries.
3. **Assumptions about other work items** — every assumption made about a document the agent did not
   write. **This is the drift detector.** If W3 assumed `Component::tick` is a single method and W2a
   decided otherwise, that contradiction is findable with two greps instead of discovered during
   implementation.
4. **Proposed amendments to `DESIGN.md`** — using the protocol below. An agent never edits a decision
   in place.

W6 consumes exactly these four sections from every document. The prose is background.

### When an agent disagrees with `DESIGN.md`

**Stop and surface it. Do not resolve it.** A single agent silently overriding a shared decision is
the specific failure mode that produces six documents which each make sense alone and do not cohere.
Write the proposed amendment, flag it at the top of the output, and carry on under the existing
decision so the document stays internally consistent.

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

### Measured request mix

Owner-supplied production traffic. **`/latest` is ~93% of all requests**, which makes it the only
endpoint whose latency materially matters.

| Endpoint | Requests | Share | Payload changes when |
|---|---|---|---|
| `/latest` | 17,740 | **~93%** | a new reading arrives (~2 s) |
| `/snow` | 499 | 2.6% — **biased low, see below** | snow cache refresh |
| `/forecast/240` | 320 | 1.7% | Aeris forecast refresh (hourly) |
| `/` | 217 | 1.1% | configuration change |
| `/forecast/24` | 136 | 0.7% | Aeris forecast refresh (hourly) |
| `/stationinfo` | 127 | 0.7% | configuration change |
| `/span/24h` | — | <1% | 1 m aggregate tick (60 s) |

Every one of these serves a payload that is **identical for all callers** and changes on a known
schedule. None of them is a per-user query. See W3 §2a.

**The `/snow` figure is not steady state.** The snow gauge was uninstalled during the measurement
window. The owner expects `/snow` to be hit at a rate comparable to `/latest` once it is reinstalled,
because the same clients poll both. **Do not treat `/snow` as negligible** — it is a second hot path,
and per request it is the *more* expensive of the two.

`/latest` currently costs **7 TimescaleDB round trips plus 1 SQLite query per request** inside a
202-line handler: latest reading, daily rain, monthly rain, yearly rain,
`get_rainfall_with_recent()` (with a fallback query if it errors), `calculate_storm_rainfall()`,
`calculate_wind_gust()`, and `GetSunTimes()`. At the measured volume that is roughly **124,000
TimescaleDB queries** recomputing answers that change at most every two seconds.

`/snow` costs **7 round trips** in a 211-line handler: a snow-cache lookup, then
`get_new_snow_midnight()`, `get_new_snow_24h()`, `get_new_snow_72h()`,
`calculate_total_season_snowfall()`, storm snowfall and snowfall rate. Several are window-scanning
PL/pgSQL; the season total scans months. Note the shape: the handler reads the snow cache **and then
computes six more values live anyway**, and `handlers.go:1418` carries a
`// TODO: Move snow calculations to almanac_cache refresh job`. The existing code already reached the
precompute conclusion and stopped halfway.

## Capability inventory

Every capability of the current system, so "did we cover everything?" is auditable rather than
remembered. Tick the right-hand column as work items claim them. Anything still marked **GAP** is
not yet owned by any prompt.

### Station drivers — note that one label covers three trigger shapes

| Capability | Shape | Owned by |
|---|---|---|
| Davis VantagePro/Vue — LOOP packets, CRC16, wake handshake, serial + TCP | serial/TCP loop | W2a §4a |
| Campbell Scientific datalogger | serial/TCP loop | W2a §4a |
| WeatherLink Live — UDP broadcast **and** HTTP poll, discovery, templates | listener + poller | W2a §4a |
| AirGradient | HTTP poller | W2a §4a |
| Ambient "customized" — vendor webhook into our HTTP listener | inbound listener | W2a §4a |
| Snow gauge | gRPC client | W2a §4a |
| gRPC receiver — inbound readings from other RW nodes | inbound listener | W2a §4a, W7 |

### Storage and collector

| Capability | Owned by |
|---|---|
| TimescaleDB writer | W3, W4 |
| gRPC stream uplink to a collector | W7 |
| Remote-station registry and registration | W7 |
| Provenance / loop prevention across multi-hop topologies | W7 §2 |
| Acked, resumable ingest with a durable spool | W7 §3 |

### Periodic and output components

| Capability | Owned by |
|---|---|
| APRS/CWOP over APRS-IS | W2a §4b |
| APRS/CWOP over **KISS TNC — serial** (AX.25) | W2a §4b |
| APRS/CWOP over **KISS TNC — TCP** | W2a §4b |
| PWS Weather upload | W2a §4b |
| Weather Underground upload | W2a §4b |
| Aeris — **forecast** fetch | W2a |
| Aeris — **alerts** fetch (separate timer) | W2a |
| Snow cache refresh | W2a |
| Rainfall summary refresh (DB job) | W4 |
| Sun-times precompute | W2a §4 |

### Services

| Capability | Owned by |
|---|---|
| Legacy REST API (frozen contract) | W1, compat item |
| gRPC `WeatherV1` service | W3 §6 |
| gRPC `WeatherAppsV1` service | W3 §6 |
| Management API | W5 |
| Public website — **multi-hostname routing** | W3 §6 |
| Public website — **per-website TLS via SNI** | W3 §6 |
| `/portal` multi-station map page | W3 §7 |
| Live updates to the browser (currently polling; SSE in the new design) | W3 §5, W5 §5 |

### Derived values — W3 §2 must decide ingest-time vs read-time for each

| Capability | Currently computed in |
|---|---|
| wind chill, heat index | `cmd/davis-instruments-forwarder` |
| AQI | `pkg/aqi` |
| rainfall day/month/year | `internal/controllers/rainfall_calc.go` |
| rainfall 24/48/72h, storm totals | PL/pgSQL |
| snow depth, new snow (PELT, RBF, median filter, dual threshold) | `internal/snow` + PL/pgSQL |
| season snowfall total | PL/pgSQL |
| circular wind-direction averaging | `circular_avg` custom aggregate |
| solar potential watts / joules | `pkg/solar` |
| lunar phase, illumination, age, crescent angle | `pkg/lunar` |
| sunrise/sunset (Meeus) | `pkg/solar` + config store |

### Configuration and operations

| Capability | Owned by |
|---|---|
| Device CRUD, enable/disable | W5 §1 |
| Storage config and per-backend health checks | W5 §1 |
| Website config, multi-site | W5 §1 |
| Radar tile token registration | W5 §1 |
| Serial-port enumeration | W5 §1 |
| WeatherLink discovery / template suggest / validate | W5 §1 |
| `/test/*` connectivity checks (device, serial, DB, storage, API, reading) | W5 §1 |
| Admin authentication — username + password, first-run setup, recovery | W5 §3 |
| Application log buffer **and** separate HTTP access log buffer | W3 §5 |
| Component dependency / `Waiting` visibility | W2a §3, W5 §2 |

### Auxiliary binaries — DECIDED: stay in Go, not ported

Owner decision: the ancillary tools stay in Go, and the dead ones are **not carried over to the new
repository at all**. Several were never used and should have lived in `scratch/` rather than `cmd/`.

Supporting evidence — only **3 of 12** `cmd/` binaries are built by `.goreleaser.yml`, so nine were
never shipped at all:

| Binary | Shipped | Doc refs | Decision |
|---|---|---|---|
| `remoteweather` | yes | — | replaced by the Rust daemon |
| `remoteweather-timescaledb-provisioner` | **yes** | 6 | **stays in Go, needs updating** — part of the install story, and must provision the new schema's requirements rather than the old tables |
| `migrate` | yes | — | **retire** with the Go daemon; superseded by `sqlx::migrate!()` |
| `davis-emulator` | no | 6 | stays in Go — **and that is better than porting**, see below |
| `campbell-emulator` | no | 6 | stays in Go, same reasoning |
| `weather-station-simulator` | no | 1 | stays in Go, same reasoning |
| `live-data-simulator` | no | 1 | stays in Go, same reasoning |
| `weather-backup` / `weather-restore` | no | 1 each | **retire** — one-time tools written for a past schema migration and rollup rebuild (owner confirmed); also hard-coded to the old schema |
| `davis-instruments-forwarder` | no | 3 | **not carried over** (owner decision) |
| `snow-calibrate` | no | 1 | **not carried over** (owner decision) |
| `moon-phase` | no | 1 | **not carried over** (owner decision) |

**Keeping the emulators in Go is actively good, not merely acceptable.** They speak the *wire
protocol* — serial frames, TCP, UDP — not the internal schema, so a Go Davis emulator exercises a
Rust Davis driver perfectly well. Because it is an independent implementation it is a *stronger*
test oracle than an emulator written by whoever wrote the driver: cross-implementation validation
for free. W3 §8 should treat them as fixed external test infrastructure rather than something to
reimplement.

**`weather-backup` / `weather-restore` are retired, not ported.** The owner confirms these were
one-time tools written to get through an earlier schema migration and the subsequent rebuild of the
roll-up tables — not an ongoing backup facility. They are also
hard-coded to the old schema (`SELECT * FROM weather`, `INSERT INTO weather`), so leaving them in Go
means they simply do not function against `observations`. They also only ever covered the raw
hypertable, never the continuous aggregates, so they were already an incomplete backup.

The resolution falls out of a decision already made: **once raw data is retained indefinitely and
the aggregates are disposable derived data, `pg_dump` of the hypertable plus "rebuild the
aggregates" *is* the backup and restore procedure.** A bespoke CSV exporter adds nothing over that,
and the aggregates need no backup because they can always be recomputed. Both tools retire rather
than being ported. **Record this in `DESIGN.md`** so nobody re-invents a backup tool, and document
the `pg_dump` + refresh procedure as the supported path.
