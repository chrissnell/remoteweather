# W0 — De-risk spike, then decisions document

**Depends on:** nothing. Everything else depends on this.
**Produces:** `docs/spike-findings.md`, then `docs/DESIGN.md`.
**Writes no production code.**

## Context

You are starting a greenfield Rust rewrite of RemoteWeather, a weather-station data collection and
publishing system. Reference repositories, both **read-only**:

- `git@github.com:chrissnell/remoteweather.git` — the Go implementation being replaced
- `git@github.com:chrissnell/RemoteWeather.app.git` — the iOS/watchOS/widget client

The order of the two parts below matters. The spike tests assumptions that the decisions document
asserts, so **run the spike first** and write the decisions informed by its results.

---

## Part 1 — `docs/spike-findings.md`

Work against a throwaway TimescaleDB instance (a container is ideal). For each question report the
command used and the measured number. Opinions are not findings.

**Pin the version deliberately.** The two load-bearing answers — `ADD COLUMN` on compressed chunks,
and hierarchical CAGGs — have changed across TimescaleDB releases, so a finding on an arbitrary
container tag does not transfer. Run the spike on the version you would actually deploy, record what
production currently runs (same `extversion` query there) and, if they differ, say which answers are
version-sensitive.

1. **Version.** `SELECT extversion FROM pg_extension WHERE extname = 'timescaledb';`

2. **The highest-risk unknown.** Does `ALTER TABLE ... ADD COLUMN <nullable>` succeed on a hypertable
   with **compressed** chunks, *without* decompressing them? How long does it take against roughly
   1 GB of compressed chunks?

   The entire "adding a metric is cheap" goal rests on this. If it fails or forces a full
   decompress, say so plainly and propose options — do not work around it silently.

3. **Real compression ratio** with `segmentby = station_id, orderby = time DESC`. Generate or sample
   ~1 GB of plausible data: slowly-varying floats, ~70% of columns NULL, a handful of stations.
   The planning assumption is 10–12×; report what you actually get.

4. **Hierarchical continuous aggregates.** Does this version support a CAGG built from another CAGG
   (1d from 1h rather than from raw)? Then prove numerical correctness: build one path that rolls up
   `sum`/`count` through the hierarchy and another that aggregates raw directly, and show with a
   query that the resulting means are identical. They will not be if a level stores a finished
   `avg`, because `avg(avg(x))` is wrong whenever bucket counts differ — which happens as soon as
   readings are missing. Confirm this empirically rather than trusting the claim.

5. **Refresh cost.** Roughly how long does a full `CALL refresh_continuous_aggregate(..., NULL, NULL)`
   take per GB of raw data?

6. **Column ceiling headroom.** Confirm the Postgres per-table column limit on this build and note
   how it interacts with a continuous aggregate that emits ~400 columns.

Open the document with a section titled **"Findings that contradict planned decisions"**. If
everything checks out, say so explicitly there.

---

## Part 2 — repository setup

**The owner has indicated a new Multica project and a new repository**, rather than a branch of the Go
tree. Treat that as the direction and do not re-argue it. The remaining questions are narrower:

- how the Go repository is referenced for behavioural comparison — submodule, sibling checkout, or
  just a documented URL (prefer the simplest; the prompts only need read access)
- where generated code lives and how it is committed so `xtask codegen --check` can gate it (W8)
- whether the frozen-contract fixtures from W1 live in the new repo, the old one, or both — they must
  be runnable against *both* binaries
- the crate/workspace layout question is W3 §1's, not yours

State these as recommendations. **Do not create any repository.**

---

## Part 3 — `docs/DESIGN.md`

The shared context every later work item reads. Record decisions **as constraints with rationale**.

**This document contains no code.** No trait definitions, no signatures, no buffer sizes, no module
layout. Those belong to W3 (service architecture). If you find yourself writing `trait` or
`async fn`, you are in the wrong document. Describe *what* and *why*; W3 decides *how*.

Include a `## Amendments` section, empty but present, with a note explaining the protocol:
decisions are never edited in place; later work items that find a decision wrong append an
amendment entry recording the date, the decision, what was found, and what replaces it. The
superseded decision stays, marked as such.

### Decisions to record

Adjust any of these if the spike contradicts them, and flag the contradiction prominently.

**Language and runtime.** Rust, tokio.

**Storage layout — wide, not narrow.** One column per metric; NOT an EAV/narrow
`(time, station, metric_id, value)` design. Rationale, which is counterintuitive and worth writing
down properly: Postgres does not store NULL values, only a bit in the row's null bitmap. A wide row
therefore costs about `42 + 4×populated` bytes regardless of how many columns are declared, while a
narrow row costs ~44 bytes *per metric* — a fresh 24-byte tuple header plus a repeated timestamp,
station id and metric id. Measured at 7–9× worse density for narrow, with no crossover point. The
wide schema's problems are maintainability, not bytes, and maintainability is solved by codegen.

**Raw retention — indefinite, with compression.** Compression enabled from day one
(`segmentby = station_id`, `orderby = time DESC`). Measured 19 GB/year uncompressed; roughly 16 GB
for ten years at 12×. The consequence is the important part: once raw data is permanent, continuous
aggregates hold no information that doesn't exist upstream, so they become **disposable derived
data** that can be dropped and rebuilt on demand. That is what makes adding a metric cheap —
`ALTER TABLE ADD COLUMN`, recreate the aggregate, refresh. Record explicitly that the versioned-CAGG
and union-view scheme was considered and **rejected** as unnecessary complexity once retention is
removed.

**Aggregates — hierarchical and composable.** 1m from raw, 1h from 1m, 1d from 1h, so rebuilding an
upper level rescans a small input instead of all of raw. This requires storing **composable**
aggregate state — `sum` and `count`, or TimescaleDB two-step `stats_agg` — never a finished `avg`.
`min`/`max`/`sum` compose natively. Note that the Go implementation already contains a correctly
composable `circular_avg_state` with a combiner function for wind direction; that pattern carries
forward, while its plain `avg` columns do not.

**One metric registry as the source of truth.** DDL, migrations, CAGG SQL, Rust types, protobuf,
OpenAPI and TypeScript types are all generated from it. Column names and protobuf field numbers are
allocated once and **never reused**; removal marks a metric deprecated rather than freeing its
identifier. Generated output is committed and CI-checked for drift. Record why: in the Go version a
metric is defined in roughly eight places, and its protobuf numbering is already damaged — station
metadata is wedged in at fields 188–192 and relays 8–10 at 193–195 because 1–187 were consumed.

**Migrations only.** Schema changes happen solely through `sqlx::migrate!()`. No DDL at startup.
Rationale: the Go version does both, and because it uses `CREATE TABLE IF NOT EXISTS` and
`CREATE MATERIALIZED VIEW IF NOT EXISTS`, its schema has been frozen at first-run shape ever since —
58 weather migrations exist and not one of them alters the hypertable or an aggregate.

**Runtime shape** (as constraints — **W2a** defines the component model, W3 the mechanics):
- Components are cancellable tasks owning their own resources, supervised as a set. There is no
  `Stop()` method; cancellation plus ownership is how something stops. Rationale: the Go
  `Controller` interface has only `StartController()`, so `RemoveController` can only delete a map
  entry and log "will stop on next app restart" — listeners and goroutines leak.
- **The word "controller" is retired.** It names a layer rather than a behaviour, which is how three
  unrelated shapes accumulated behind one interface. Replaced by a single `Component` abstraction in
  which *when it runs* and *what it needs* are declarative. See W2a.
- **A component declares a trigger, not its own timer.** Of eleven existing long-running components
  only two are driven by reading arrival; the rest are periodic or long-running services, and none
  of them subscribe to readings — they poll the database on hand-rolled tickers. The supervisor owns
  all scheduling, which makes "when did this last run" inspectable state.
- **Dependencies between components are declared, not implicit.** Components state the capabilities
  they require and provide, and the supervisor resolves a startup order. Rationale: today
  `/forecast` and `/alerts` silently depend on the Aeris component, sunrise/sunset depends on a
  precompute buried in the REST controller's constructor, APRS depends on the station registry, and
  the management API depends on the application object itself — a genuine cycle.
- **A missing dependency means waiting, not refusing.** Four Go controllers call
  `ValidateTimescaleDBConfig` and refuse to construct without it, so a transient database problem at
  boot disables the uploaders until the process restarts. Components must instead sit in a `Waiting`
  state and start when the capability becomes healthy.
- Configuration is distributed to components by a watch/broadcast mechanism, not shared mutable
  state. Rationale: the Go REST controller holds `WeatherWebsites`, `Devices`, `DeviceNames`,
  `DevicesByWebsite` and `tlsConfigs` as plain fields with **no mutex**, replaced wholesale by the
  management API while every in-flight request reads them. In Go a concurrent map read/write is an
  unrecoverable fatal error, so saving a website while serving traffic can kill the process.
- Data flows sources → fan-in → transform/enrich → fan-out → sinks, with an explicit policy for what
  happens when a sink can't keep up. Rationale: the Go reading distributor does a blocking unguarded
  send per engine, so a stalled database back-pressures the serial read loops and can hang shutdown.
- Sinks are composable and carry per-sink delivery policy. Durable sinks must not lose data;
  rate-limited upload sinks should coalesce to the newest reading, because backfilling Weather
  Underground after an outage is wrong.
- Transport is orthogonal to role. gRPC and REST are codecs, not kinds of component. Rationale: in
  the Go version egress-to-a-peer is implemented as a *storage engine* and ingress-from-a-peer as a
  *weather station*, so the collector role has no name anywhere in the codebase.

**Named crates for the idioms below.** Record these so six agents reach for the same ones rather than
each choosing: `tokio` + `tokio-util` (`CancellationToken`, `codec`), `tower` for sink composition and
rate limiting, `axum` + `tonic` sharing one listener, `sqlx` for both databases, `prost` + `pbjson`
so REST-JSON and gRPC-protobuf are generated from **one** schema (replacing the ~591 hand-written
mapping lines in `grpcstream/conversion.go` and `restserver/weatherapps_grpc.go`), `thiserror` +
`anyhow` for errors, `secrecy` for secret wrapper types, `subtle` for constant-time comparison, `argon2` for password hashing,
`zerocopy` for fixed-layout binary packet decode (Davis LOOP is a fixed little-endian struct currently
decoded via reflection-based `binary.Read`), and `arc-swap` or `tokio::sync::watch` for published
snapshots. Any substitution is fine but must be recorded as an amendment, because divergence here is
exactly what W6 has to reconcile.

**Secrets and errors.** Secrets use a wrapper type whose `Debug`/`Display` redact; credential
comparison is constant-time. Rationale: the current management token is a UUID compared with `==`,
printed at startup and on every failed auth attempt, with the session cookie set to the token
itself. Errors use `thiserror` per module with `anyhow` at the binary boundary, and `#[from]` so
chains survive — the Go version uses `%v` in 767 of 1,174 `fmt.Errorf` sites, destroying them.

**Backup and restore is `pg_dump`, not a bespoke tool.** Because raw data is retained indefinitely
and the aggregates are disposable derived data, the supported procedure is a `pg_dump` of the
hypertable plus an aggregate refresh on restore. The Go `weather-backup`/`weather-restore` pair is
retired rather than ported: it was hard-coded to `SELECT * FROM weather`, and it never covered the
continuous aggregates, so it was an incomplete backup even for the old schema. Document the
procedure; do not build a tool.

**Ancillary Go tools are not ported.** Only three of twelve `cmd/` binaries were ever shipped. The
provisioner stays in Go and needs updating for the new schema; `migrate` retires with the Go daemon;
the four emulators/simulators stay in Go deliberately, as independent-implementation test
infrastructure for the Rust drivers; the rest are unused and belong in `scratch/`.

**Admin authentication is a username and password.** Owner decision: one administrator, no RBAC and
no multiple accounts; Argon2id password storage; sessions distinct from credentials; a defined
first-start password-setup process plus a local CLI recovery path; no default password ever. W5 §3
owns the design. Rationale for replacing what exists: the current scheme is a UUID bearer token
compared with `==`, printed in cleartext at startup *and* on every failed attempt, with the session
cookie set to the token itself so a browser session cannot be revoked without rotating the API
credential. Note this touches only the management API — the public API is unauthenticated and frozen
by W1, so there is no contract risk.

**Internal vocabulary is not user-facing vocabulary.** The component model's names are chosen for the
code. The management UI maintains its own plain-language vocabulary with an explicit translation
table (W5 §0), and an internal term appearing on screen is treated as a bug. Rationale: the owner
reports that "controller" and "storage backend" are already not obvious to new users, and the new
internal names would be worse on screen, not better.

**Frozen external contract.** The iOS app and the existing weather website must keep working
unchanged; new endpoints are additive only. The public website is carried forward as-is rather than
redesigned. W1 captures the exact surface; do not attempt to enumerate it here.

**Testing and CI.** A PR gate that runs build, test, `fmt`, clippy and `-race`-equivalent concurrency
tests exists from the first commit. Rationale: the Go repo's only workflow is release-on-tag, which
is why 45 files fail `gofmt` and the two data races above went unnoticed.

---

## Do not

- Write production Rust.
- Create repositories, or modify either reference repo.
- Design the metric registry format — that is W2.
- Put trait definitions or signatures in `DESIGN.md` — that is W3.
- Enumerate the legacy API surface — that is W1.
- Touch the production database. The spike runs against a throwaway instance only.

## Done when

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`: **Decisions** (numbered one-liners), **Open questions** (with
  recommendations), **Assumptions about other work items**, and **Proposed amendments to
  `DESIGN.md`**. W6 consumes these; a document without them cannot be reconciled.

- Every spike question has a measured answer with the command that produced it.
- The "findings that contradict planned decisions" section is filled in, even if only to say nothing
  contradicted.
- `DESIGN.md` records each decision with its rationale, contains no code, and has an `Amendments`
  section with the protocol described.
- The repository recommendation is stated with reasoning.
