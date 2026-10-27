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

## Capability inventory

Every capability of the current system, so "did we cover everything?" is auditable rather than
remembered. Tick the right-hand column as work items claim them. Anything still marked **GAP** is
not yet owned by any prompt.

### Station drivers — note that one label covers three trigger shapes

| Capability | Shape | Owned by |
|---|---|---|
| Davis VantagePro/Vue — LOOP packets, CRC16, wake handshake, serial + TCP | serial/TCP loop | W2a, drivers item |
| Campbell Scientific datalogger | serial/TCP loop | W2a, drivers item |
| WeatherLink Live — UDP broadcast **and** HTTP poll, discovery, templates | listener + poller | W2a, drivers item |
| AirGradient | HTTP poller | W2a, drivers item |
| Ambient "customized" — vendor webhook into our HTTP listener | inbound listener | W2a, drivers item |
| Snow gauge | gRPC client | W2a, drivers item |
| gRPC receiver — inbound readings from other RW nodes | inbound listener | W2a, collector item |

### Storage and collector

| Capability | Owned by |
|---|---|
| TimescaleDB writer | W3, W4 |
| gRPC stream uplink to a collector | W3, collector item |
| Remote-station registry and registration | collector item |
| Provenance / loop prevention across multi-hop topologies | W3 §2, collector item |
| Acked, resumable ingest with a durable spool | collector item |

### Periodic and output components

| Capability | Owned by |
|---|---|
| APRS/CWOP over APRS-IS | W2a |
| APRS/CWOP over **KISS TNC — serial** (AX.25) | W2a |
| APRS/CWOP over **KISS TNC — TCP** | W2a |
| PWS Weather upload | W2a |
| Weather Underground upload | W2a |
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
| Auth and token rotation | W5 §3 |
| Application log buffer **and** separate HTTP access log buffer | W3 §5 |
| Component dependency / `Waiting` visibility | W2a §3, W5 §2 |

### Auxiliary binaries — DECIDED: stay in Go, not ported

Owner decision: the ancillary tools stay in Go. Several are no longer used and should have lived in
`scratch/` rather than `cmd/`.

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
| `weather-backup` / `weather-restore` | no | 1 each | **retire** — will not work against the new schema, see below |
| `davis-instruments-forwarder` | no | 3 | unused → `scratch/` or delete |
| `snow-calibrate` | no | 1 | unused → `scratch/` or delete |
| `moon-phase` | no | 1 | unused → `scratch/` or delete |

**Keeping the emulators in Go is actively good, not merely acceptable.** They speak the *wire
protocol* — serial frames, TCP, UDP — not the internal schema, so a Go Davis emulator exercises a
Rust Davis driver perfectly well. Because it is an independent implementation it is a *stronger*
test oracle than an emulator written by whoever wrote the driver: cross-implementation validation
for free. W3 §8 should treat them as fixed external test infrastructure rather than something to
reimplement.

**`weather-backup` / `weather-restore` need a different answer than "leave in Go."** They are
hard-coded to the old schema (`SELECT * FROM weather`, `INSERT INTO weather`), so leaving them in Go
means they simply do not function against `observations`. They also only ever covered the raw
hypertable, never the continuous aggregates, so they were already an incomplete backup.

The resolution falls out of a decision already made: **once raw data is retained indefinitely and
the aggregates are disposable derived data, `pg_dump` of the hypertable plus "rebuild the
aggregates" *is* the backup and restore procedure.** A bespoke CSV exporter adds nothing over that,
and the aggregates need no backup because they can always be recomputed. Both tools retire rather
than being ported. **Record this in `DESIGN.md`** so nobody re-invents a backup tool, and document
the `pg_dump` + refresh procedure as the supported path.
