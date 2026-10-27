# W3 — Service architecture design

**Depends on:** W0 and **W2a (the component and capability model)**. Runs in parallel with W1 and W2.
**Produces:** `docs/service-architecture.md`.
**Writes no implementation code.** Trait signatures and type sketches only — enough to prove the
design coheres, not a working crate.

## Context

Read `docs/DESIGN.md` first. It records decisions already made; do not re-litigate them. If you find
one wrong, append an amendment entry per the protocol in that document.

Read `docs/component-model.md` (W2a) second. It defines the `Component` trait, the `Trigger` enum
and the `Capability` set that everything here plugs into. **Do not redesign it.** If it is wrong,
raise that against W2a rather than working around it.

`DESIGN.md` owns the *what*. W2a owns **what a component is**. **This document owns the *how*** —
signatures, module boundaries, buffer sizes, policies.

The Go implementation at `git@github.com:chrissnell/remoteweather.git` is a read-only reference for
*behaviour*. Assume its *structure* is what we are replacing; where it is cited below, it is a
cautionary tale.

**On the metric registry:** assume `Reading` and `Metric` exist with the conceptual shape described
in `DESIGN.md`. Their file format and generation are W2's problem. Where your design needs to
reference how they're produced, defer to W2 by name rather than inventing a format.

---

## 1. Crate and module layout

Propose the structure and justify the boundaries. For each crate or module, state its single
responsibility and what it is **forbidden to depend on**.

Decide explicitly:
- cargo workspace or single crate with modules, and why
- where generated code lives, and how it stays out of hand-written modules
- where the legacy-compat adapter lives, and how its boundary prevents legacy field naming and
  zero-filling semantics from leaking into the core domain types

**Anti-goal to design against.** The Go `internal/controllers/restserver/Controller` is a single type
owning an HTTP server, an HTTPS server, a gRPC server, the gRPC service implementation, SNI
certificate management, middleware, database queries and config reload. Nothing in the new design
should be able to accrete like that. Say specifically how your boundaries prevent it.

## 2. The reading pipeline — the `OnReading` path

Scope note, because this is easy to get wrong: the pipeline is **one trigger among six**, not the
organising principle of the system. Per W2a, only the storage writer and the collector uplink are
driven by reading arrival; APRS, PWS, Weather Underground, Aeris and the snow cache are periodic,
and the servers are long-running services. Design this section as the implementation of the
`ReadingStream` capability and the `OnReading` trigger — not as the shape of the whole service.

**Traits.** Exact definitions for `Source` and `Sink`, including how cancellation, backpressure and
failure propagate.

**Fan-in and fan-out.** The stages, buffer sizes with reasoning, and the policy for a slow consumer.
A lagging receiver must be handled explicitly rather than blocking the producer — decide the policy
*per sink kind* and say why it differs.

Reference failure: the Go reading distributor does an unguarded blocking send per engine with no
`select` and no context check, so a stalled TimescaleDB back-pressures the serial read loops and a
blocked send hangs shutdown forever. Its engine list is also mutated without a mutex while the
distributor iterates it, and `RemoveEngine` closes a channel the distributor may be sending on.

**The transform/enrich stage — which the Go version does not have.** Derived values are currently
computed in scattered places:

| Derived value | Currently computed in |
|---|---|
| wind chill, heat index | `cmd/davis-instruments-forwarder` (with a TODO acknowledging it's missing elsewhere) |
| AQI | `pkg/aqi` |
| rainfall accumulation (day/month/year) | `internal/controllers/rainfall_calc.go`, called from HTTP handlers |
| rainfall 24/48/72h, storm totals | PL/pgSQL functions inside the database |
| snow depth, new-snow deltas, season totals | `internal/snow` **and** PL/pgSQL functions |

Decide for **each** derived value whether it is computed once at ingest and stored, or computed at
read time in SQL — and state the rule that makes that choice non-arbitrary rather than per-case
taste. The current inconsistency is one of the concrete bad ideas being ditched, so a design that
leaves it unresolved has not done this section.

**Provenance.** How an `Envelope` carries origin, path and sequence, and how loop prevention becomes
a structural invariant rather than a name-matching filter.

Reference failure: the Go collector prevents echo loops by calling `LoadConfig()` **on every single
reading** and comparing the reading's station name against the names of local `grpcreceiver`
devices. Since received readings preserve the *origin's* name, the guard only fires on a naming
coincidence — so cycles are not reliably stopped, and multi-hop relay is not expressible at all.

## 2a. The read path and the hot-path snapshot

Owner-supplied production traffic (recorded in `docs/rewrite/README.md`) shows **`/latest` is ~93% of
all requests**. It is the only endpoint whose latency materially matters, and the measured mix has a
property that determines the whole read-path design: **every top endpoint serves a payload that is
identical for all callers and changes on a known schedule.** None is a per-user query.

Today `/latest` costs **7 TimescaleDB round trips plus 1 SQLite query per request** in a 202-line
handler — latest reading, daily/monthly/yearly rain, `get_rainfall_with_recent()` (plus a fallback
query on error), `calculate_storm_rainfall()`, `calculate_wind_gust()`, and `GetSunTimes()`. At the
measured volume that is ~124,000 queries recomputing answers that change at most every two seconds.

**Serving is not sinking — three distinct shapes.** Storage-as-a-sink is right for the *write* path,
but the read path is a different concern and must not be modelled as a sink:

```
write:  Sources ──mpsc──▶ pipeline ──broadcast──▶ Sinks     (TimescaleDB writer, uplink, uploaders)
read:   REST/gRPC ──────▶ ReadingStore                      (spans, almanac, aggregates)
live:   REST/gRPC stream ──▶ a tap on the broadcast         (GetLiveWeather, SSE)
```

So `TimescaleSink` (write) and `ReadingStore` (read) are two traits over one connection pool, because
the read path's concerns — aggregate selection, pagination, caching — have nothing to do with delivery
policy. And `GetLiveWeather` is neither: it is a subscriber to the same broadcast the sinks use. Name
the three separately so nobody later tries to make the website a sink.

**Design requirement: nothing in the hot path touches the database.** Specify a `LatestSnapshot`
capability — a component with `Trigger::OnReading` that rebuilds the response payload when a reading
arrives and publishes it behind an `ArcSwap`/`watch`. The handler's entire job becomes: resolve the
website, select the station, clone an `Arc`, write bytes.

Specify:

- **Pre-serialise, don't just cache the struct.** The payload is byte-identical for all callers, so
  serialise once per *reading* rather than once per *request*. The legacy response has 133 fields, so
  this is not free work to repeat 17,000 times.
- **Snapshot cardinality and keying** — per `(website, station)`. A handful of each, a few KB apiece,
  so memory is a non-issue; say so explicitly rather than leaving it as a worry.
- **Conditional GET.** The payload changes only when a reading arrives, so an ETag derived from the
  reading timestamp lets the iOS app's polling receive 304s. This must stay backward compatible: a
  client that sends no `If-None-Match` gets a normal 200 with the full body. Additive only — the
  contract in W1 is frozen.
- **Graceful degradation.** With `/latest` served from memory, a database outage no longer fails 93%
  of traffic; serve the last snapshot with a staleness indicator. Today a blip breaks the dominant
  endpoint outright.
- **`/snow` is a second hot path, not part of "the rest."** Its measured 2.6% share reflects an
  uninstalled snow gauge; the owner expects a rate comparable to `/latest` once it is back, from the
  same polling clients. Per request it is more expensive than `/latest`: 7 round trips in a 211-line
  handler, several of them window-scanning PL/pgSQL, with `calculate_total_season_snowfall()` scanning
  months. It therefore needs the snapshot treatment at least as much as `/latest`.

  It also needs a **two-tier** snapshot, which is why `SnowCache` is a separate capability from
  `LatestSnapshot` rather than a special case of it:

  | Tier | Values | Trigger |
  |---|---|---|
  | cheap / fast-changing | current depth, snow distance | `OnReading` |
  | expensive / slow-changing | midnight, 24 h, 72 h, season total, storm total, snowfall rate | `Every(n minutes)` |

  Snow accumulates over hours, so recomputing a season total per reading — let alone per request — is
  pure waste. Specify the refresh interval and say what the UI shows for snapshot staleness.

  Supporting evidence that this is the right shape: the Go tree already has a `snowcache` controller
  *because* these are expensive, the handler queries it, and then computes six more values live
  anyway. `handlers.go:1418` says `// TODO: Move snow calculations to almanac_cache refresh job`. The
  design was already heading here.

- **The rest of the mix.** `/snow` aside, `/forecast/*` from
  `ForecastStore` (hourly refresh, so a snapshot); `/` as a rendered-page cache invalidated by the
  config watch; `/stationinfo` from `ConfigStore`. `/span/24h` is the only genuinely query-shaped
  endpoint and is under 1% of traffic — a 60-second TTL is lossless, because the 1 m aggregate only
  advances once a minute.

**This supplies the rule §2 asks for.** The seven queries exist because day/month/year rain,
24/48/72 h rain, storm totals and wind gust are all computed at read time. The traffic mix resolves
the ingest-versus-read question non-arbitrarily:

> Anything on the `/latest` payload is computed at ingest and lives in the snapshot. Nothing on the
> `/latest` payload is computed per request.

Use that as the stated rule, and apply it to the derived-values table in §2.

**What not to do**, since "optimise the hot path" invites gold-plating. At 17,740 requests the
absolute rate is low — this is about tail latency and about not generating 124,000 redundant queries,
**not** about throughput headroom. So: no Redis or memcached (the data is tiny and single-process; an
`Arc` *is* the cache), no CDN (the payload changes every two seconds, so TTLs buy nothing and it adds
an operational dependency), no hard optimisation of `/span/24h`, and no hand-tuning the JSON encoder
once serialisation is off the per-request path.

## 3. Supervisor implementation

W2a specifies the lifecycle *states* (`Waiting`/`Running`/`Failed`/`Backoff`/`Stopped`), capability
resolution and startup ordering. This section specifies the **mechanics**: how tasks are spawned,
cancelled and awaited; how the supervisor owns and fires every timer for `Every`, `Calendar` and
`OnStartup` triggers; and how a component's `tick` returning `Err` maps onto the state machine.

State the invariant that makes a leaked task or an un-awaited handle **impossible to express**.

Reference failures: the Go `Controller` interface has only `StartController()`, so `RemoveController`
deletes a map entry and logs "will stop on next app restart"; the APRS controller constructs its own
private `sync.WaitGroup` so its goroutines are invisible to app shutdown and it receives no context
at all; and three sites call `wg.Add(1)` from *inside* a function already launched with `go`, so
`wg.Wait()` can return before they register.

## 4. Error taxonomy

Module-level error enums. What is retryable, fatal, or ignorable, and how that classification drives
the retry layers instead of being re-decided at each call site. Specify how `#[from]` and `?` make
chain-preserving the default path.

## 5. Logging and observability

A first-class deliverable, not a footnote. Requirements:

- **`tracing`, one logging path only.** The Go version has both a package-global sugared logger and
  an injected logger, used interchangeably within the same functions. Do not reproduce that.
- **Pretty for humans, structured for machines.** ANSI and aligned when stderr is a TTY; JSON when
  it isn't (journald, Docker). Decide via detection plus an override flag.
- **Spans must make per-station filtering trivial.** Every reading, sink delivery and request
  carries station/origin so an operator can isolate one device's entire story.
- **Runtime-adjustable levels** via `tracing_subscriber::reload`, so the management UI can raise one
  module's level without a restart. Specify the mechanism and its concurrency story.
- **Two buffers, not one.** The current system keeps the application log and the HTTP access log in
  *separate* buffers, surfaced at `/logs` and `/http-logs`. Decide whether to keep them separate or
  unify with a filterable field, and say why.
- **A ring-buffer layer for the management UI.** A custom `Layer` writing structured events into a
  bounded in-memory buffer, consumed over SSE. Contrast with the Go approach, which serialises zap
  output to JSON and then **re-parses it back into structs** to feed the UI.
- **Repetition control.** A disconnected serial port must not emit thousands of identical lines an
  hour. Specify rate-limiting or dedupe-with-count for recurring errors. This is the difference
  between logs that are useful and logs nobody reads.
- **Actionable messages.** Set the standard: a dropped reading names the station, the metric and the
  reason; a sink failure names the sink, how many envelopes, and whether they were spooled or lost.
- **Metrics are not logs.** Decide Prometheus or counters on a status endpoint, and name the handful
  of numbers that matter — readings ingested per station, sink lag and spool depth, dropped-reading
  count by reason, aggregate refresh duration.

## 6. The HTTP layer

One listener, not seven. Per W2a, components that serve HTTP **provide route tables** via the
`HttpRoutes` capability and a single server component mounts them — the website, the management API,
the REST ingest endpoint and the legacy compatibility routes all land on one socket. Specify that
mounting mechanism: how a route table is declared, how conflicts are detected, and whether detection
happens at startup or at build time.

Today listener construction is spread across seven files, each with its own TLS setup and shutdown
path: `restserver/controller.go` (5), `storage/grpcstream/grpc.go` (3),
`weatherstations/grpcreceiver/station.go` (2), `controllers/management/controller.go` (2),
`weatherlinklive/udp_receiver_{unix,windows}.go` and `ambientcustomized/station.go`.

**Multi-website hosting is a real capability and must survive.** The current REST server routes by
`Host` header to a per-website configuration, and serves a different site per hostname from one
process. Specify:

- how the hostname → website lookup works, and how it reads reload-able config without the data race
  the Go version has (`WeatherWebsites` is a plain map replaced wholesale while requests read it)
- what happens for an unmatched `Host`, and the default-website fallback
- **per-website TLS via SNI.** `getCertificate` currently looks up `tlsConfigs[hello.ServerName]` to
  pick a certificate per hostname at handshake time. Specify how certificates are loaded, keyed,
  reloaded when config changes, and what happens on SNI miss. Certificate paths come from the config
  store, so a reload must be able to swap a certificate without dropping the listener.
- **one schema, two encodings.** REST-JSON and gRPC-protobuf must be generated from a single
  definition — `prost` plus `pbjson` — so they cannot drift. This replaces ~591 lines of
  hand-maintained mapping in the Go tree (`grpcstream/conversion.go` at 276 and
  `restserver/weatherapps_grpc.go` at 315). Keep the wire type distinct from the domain type with one
  `TryFrom` at the boundary, shared by every transport; that is where units and `Option` live.
- how gRPC coexists on the same socket (ALPN), given that gRPC must remain available for
  machine-to-machine peers while browsers use REST and SSE


## 7. Serving the frozen public website

The existing weather website is carried forward **as-is**, and this document owns how. W1 captures
what it currently emits; you decide the mechanism.

Three Go templates exist: `portal.html.tmpl`, `weather-station.html.tmpl`, and
`js/weather-app.js.tmpl`. Decide and justify:

- port the templates to a Rust engine (askama/maud, compile-time checked) versus pre-render versus
  another approach — visual output must not change
- how static assets are served, and the cache-header strategy
- **`weather-app.js.tmpl` is replaced — this is decided, not a recommendation.** It is JavaScript
  generated by Go's `text/template`, which is a code-injection surface. **Owner decision: replace it
  with properly written static JavaScript plus a JSON configuration endpoint.** Same rendered result,
  no template-into-JS step. This is the one deliberate exception to "the website is carried forward
  as-is" — specify the config endpoint's shape and confirm the rendered page is visually unchanged.
- `AboutStationHTML` is injected into pages as raw unescaped HTML from the config database. Say how
  that is handled, and what the trust boundary is.
- the module boundary that keeps all of the above from leaking into the new API or domain types

## 8. Testing strategy

Per layer: what is a unit test, what needs a real TimescaleDB container, what needs captured
hardware frames. State how a driver is tested without the physical device — the Go repo ships four
emulators in `cmd/`, which is evidence this matters.

Specify the concurrency tests that would have caught the Go version's actual bugs: config reload
racing live readers, and shutdown with a stalled sink.

## 9. Extensibility walkthroughs

Prove the design by tracing three changes end to end, naming every file touched:

1. add a new weather station driver (cover all three trigger shapes it might take — serial loop,
   HTTP poller, inbound listener)
2. add a new stream sink (e.g. an MQTT publisher)
3. add a new **periodic** component (e.g. another upload service like PWS/WU). This is the case the
   owner cares about most, and the one the Go version handles worst — pwsweather and wunderground
   are line-for-line identical because there was no shared shape for it.
4. add a new metric

If any of these touches more than a handful of files, the design is not finished. For calibration,
adding a metric in the Go version touches roughly eight places. For walkthrough 3, the generation
half belongs to W2 — reference it rather than designing it.

## 10. Efficiency boundaries

State explicitly what is **not** worth optimising. This system handles a few readings per second,
and the measured request mix (README) is ~19,000 requests dominated by one endpoint — so the absolute
rate is low. The only things that matter are `/latest` tail latency (§2a), on-disk compression ratio,
and not stalling ingest on a slow sink. Name the places where someone might be tempted to micro-optimise, and say
don't.

---

## Do not

- Write implementation code.
- Design the metric registry file format — that is W2.
- Design the management UI — that is W5.
- Design the new v2 API surface — later work item. The *legacy* contract is a frozen input here.
- Redesign the public website's appearance or behaviour. Section 6 is about serving it, not changing
  it.

## Done when

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`: **Decisions** (numbered one-liners), **Open questions** (with
  recommendations), **Assumptions about other work items**, and **Proposed amendments to
  `DESIGN.md`**. W6 consumes these; a document without them cannot be reconciled.

- Every trait signature at a pipeline boundary is specified.
- Every derived value in the section 2 table has an ingest-or-read decision, and the governing rule
  is stated.
- All three extensibility walkthroughs name their touched files.
- Logging requirements are concrete enough to implement without further decisions.
- §2a specifies the `LatestSnapshot` capability, pre-serialisation, ETag behaviour and the
  ingest-versus-read rule, and applies that rule to the §2 derived-values table.
- Section 6 specifies hostname routing and the SNI certificate story.
- Section 7 makes a recommendation on each of the website-serving questions.
- Any conflict with `DESIGN.md` is recorded as an amendment rather than silently resolved.
