# W3 — Service architecture design

**Depends on:** W0. Runs in parallel with W1 and W2.
**Produces:** `docs/service-architecture.md`.
**Writes no implementation code.** Trait signatures and type sketches only — enough to prove the
design coheres, not a working crate.

## Context

Read `docs/DESIGN.md` first. It records decisions already made; do not re-litigate them. If you find
one wrong, append an amendment entry per the protocol in that document.

`DESIGN.md` owns the *what*. **This document owns the *how*** — signatures, module boundaries,
buffer sizes, policies.

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

## 2. The source → transform → sink pipeline

The core of this document.

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

## 3. Lifecycle and supervision

How tasks are spawned, cancelled and awaited. What happens when a source or sink returns `Err`.
Restart-with-backoff policy and where that decision lives.

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

## 6. Serving the frozen public website

The existing weather website is carried forward **as-is**, and this document owns how. W1 captures
what it currently emits; you decide the mechanism.

Three Go templates exist: `portal.html.tmpl`, `weather-station.html.tmpl`, and
`js/weather-app.js.tmpl`. Decide and justify:

- port the templates to a Rust engine (askama/maud, compile-time checked) versus pre-render versus
  another approach — visual output must not change
- how static assets are served, and the cache-header strategy
- **`weather-app.js.tmpl` is JavaScript generated by Go's `text/template`.** That is a code-injection
  surface. Recommend whether to carry it forward verbatim or replace it with static JS plus a JSON
  config endpoint — same rendered result, no template-into-JS step. Make a recommendation; the call
  is the owner's.
- `AboutStationHTML` is injected into pages as raw unescaped HTML from the config database. Say how
  that is handled, and what the trust boundary is.
- the module boundary that keeps all of the above from leaking into the new API or domain types

## 7. Testing strategy

Per layer: what is a unit test, what needs a real TimescaleDB container, what needs captured
hardware frames. State how a driver is tested without the physical device — the Go repo ships four
emulators in `cmd/`, which is evidence this matters.

Specify the concurrency tests that would have caught the Go version's actual bugs: config reload
racing live readers, and shutdown with a stalled sink.

## 8. Extensibility walkthroughs

Prove the design by tracing three changes end to end, naming every file touched:

1. add a new weather station driver
2. add a new sink (e.g. an MQTT publisher)
3. add a new metric

If any of these touches more than a handful of files, the design is not finished. For calibration,
adding a metric in the Go version touches roughly eight places. For walkthrough 3, the generation
half belongs to W2 — reference it rather than designing it.

## 9. Efficiency boundaries

State explicitly what is **not** worth optimising. This system handles a few readings per second.
The only things that matter are website query latency, on-disk compression ratio, and not stalling
ingest on a slow sink. Name the places where someone might be tempted to micro-optimise, and say
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

- Every trait signature at a pipeline boundary is specified.
- Every derived value in the section 2 table has an ingest-or-read decision, and the governing rule
  is stated.
- All three extensibility walkthroughs name their touched files.
- Logging requirements are concrete enough to implement without further decisions.
- Section 6 makes a recommendation on each of the website-serving questions.
- Any conflict with `DESIGN.md` is recorded as an amendment rather than silently resolved.
