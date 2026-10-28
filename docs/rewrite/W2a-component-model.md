# W2a — The component and capability model

**Depends on:** W0. Runs in parallel with W1 and W2. **W3 and W5 depend on this.**
**Produces:** `docs/component-model.md`.
**Writes no implementation code.** Trait and enum sketches only.

## Why this is its own work item

The Go implementation calls everything in this layer a "controller," which names a *layer* rather
than a *behaviour*. As a result three unrelated shapes accumulated behind one interface
(`StartController() error`), and the relationships between them were never expressed.

Measured from the existing code, of eleven long-running components **two** are driven by reading
arrival. Every other one is periodic or a long-running service, and **not one of them subscribes to
the reading stream** — they all poll the database on their own ticker:

| Component | Trigger today | Reads from | On the reading stream? |
|---|---|---|---|
| TimescaleDB writer | reading arrival | — | **yes** |
| gRPC stream uplink | reading arrival | — | **yes** |
| APRS / CWOP | `time.NewTicker` | DB + station registry | no |
| PWS Weather | `time.NewTicker` | DB | no |
| Weather Underground | `time.NewTicker` | DB | no |
| Aeris — forecasts | `time.NewTicker` | outbound HTTP | no — it *produces* |
| Aeris — alerts | a **second** `time.NewTicker` | outbound HTTP | no — it *produces* |
| Snow cache | `time.NewTicker` | DB → DB | no |
| REST/gRPC server | own accept loop | DB, forecasts, alerts, snow, sun times | no |
| Management API | own accept loop | config, logs, the app object | no |
| Sun-times precompute | **once, at startup** | config | no |

So a design organised around "sources and sinks on a reading stream" describes the ingest path and
mislabels everything else. Modelling APRS as a sink would make it receive every reading in order to
discard nearly all of them.

There is also a dependency graph that already exists but is expressed only through constructor
arguments and startup ordering:

- `/forecast` and `/alerts` depend on the Aeris component having populated storage. Nothing states
  or enforces this.
- Sunrise/sunset depend on `PopulateSunTimes` having run — and that precompute is buried inside the
  REST controller's own constructor, iterating devices at startup.
- `/snow` depends on the snow cache component.
- APRS depends on the **station registry** for coordinates and callsign. It is the only controller
  that takes `stationManager`, and also the only one given neither `ctx` nor `wg`.
- APRS, Aeris, PWS, Weather Underground and the snow cache all depend on TimescaleDB. **Four of them
  call `ValidateTimescaleDBConfig` and refuse to construct without it**, so a transient database
  problem at boot permanently disables the uploaders until the process restarts.
- The management API depends on `AppReloader` — the application object that owns it. A genuine
  cycle, and the reason the management controller is special-cased out of the controller manager
  entirely.

## Scope

Design the model that replaces "controller." Retire the word.

**One constraint on naming.** The vocabulary chosen here (`Component`, `Capability`, `Trigger`,
`Source`, `Sink`) is **internal only**. It is better for the code and worse for the user —
"Component APRS is Waiting on Capability ReadingStore" is less comprehensible than what exists
today. W5 §0 owns the user-facing translation and forbids these terms from reaching the screen. Do
not optimise these names for display; optimise them for the code, and make sure every lifecycle
state and trigger carries enough structure for W5 to render it in plain language.

### 1. The component abstraction

One trait, no kind taxonomy. The things that vary are **when a component runs** and **what it
needs** — make both declarative rather than encoded in each component's internals.

Starting sketch, to be refined or replaced with justification:

```rust
trait Component {
    fn id(&self)       -> ComponentId;
    fn trigger(&self)  -> Trigger;
    fn requires(&self) -> &[Capability];
    fn provides(&self) -> &[Capability];
    async fn tick(&mut self, cx: &Cx) -> Result<()>;
}

enum Trigger {
    OnReading(Filter),        // storage writer, collector uplink
    Every(Duration),          // PWS, WU, Aeris, snow cache
    Calendar(CronSpec, Tz),   // rollovers at local midnight
    OnStartup,                // sun-times precompute
    OnDemand,                 // only via API
    Service,                  // owns its own loop: listeners
}
```

Decide whether a single `tick` method is the right shape for all six triggers, or whether `Service`
needs a different entry point. Justify either way — a trait that lies about one variant is worse
than two traits.

**The supervisor owns every timer.** No component constructs its own ticker. Today each one
hand-rolls a ticker plus an initial delay plus its own error loop, which is why
`sendPeriodicReports` in pwsweather and wunderground is line-for-line identical. Beyond the
duplication, the point is that "when does this run, when did it last run, when does it run next"
becomes inspectable state rather than something buried in a goroutine.

`Every(Duration)` is deliberately insufficient for rainfall day/month/year rollovers, which reset at
**local midnight** — devices carry a `timezone` field. Specify how `Calendar` handles timezones,
DST transitions, and a missed window (process down across midnight).

Two further things `Every` must define, both load-bearing for the rate-limited outputs:

- **Overlap policy.** What happens when a tick fires while the previous one is still running (slow
  network, hung TCP session). For rate-limited outputs the answer is **skip and count the skip**, not
  queue — queuing builds a backlog that later fires as a burst, which is exactly what the rate limit
  exists to prevent. Surface the skip count, because repeated skips mean the configured interval is
  shorter than the round-trip.
- **Jitter.** Because the supervisor owns all timers, per-instance jitter is applied centrally. Today
  every uploader does "initial upload in 15 seconds" and then fires on aligned intervals, so they
  stampede together against both the local database and the upstream services.

### 2. Capabilities and the dependency graph

Define the capability set and the resolution rules. Starting point:

```rust
enum Capability {
    ReadingStream, ReadingStore, StationRegistry, ConfigStore,
    ForecastStore, AlertStore, SnowCache, SunTimes, HttpRoutes,
    LatestSnapshot,   // the hot path -- see W3 §2a
}
```

Specify:

- how `requires`/`provides` are resolved into a startup order, and what happens on a cycle
  (detected at startup and fatal, or detected at build time?)
- how a capability is handed to a component — typed handle, registry lookup, something else
- **how the `AppReloader` cycle is eliminated.** Reconfiguration should be a capability backed by
  the config watch channel, not a back-reference to the application object.
- `LatestSnapshot` as a capability. `/latest` is ~93% of production traffic and currently costs 7
  TimescaleDB round trips per request. A component with `Trigger::OnReading` must provide a
  pre-serialised snapshot so the handler performs no queries at all. W3 §2a owns the mechanics; this
  document owns the capability's place in the graph and the fact that the HTTP server component
  *requires* it.
- `HttpRoutes` as a capability: components that serve HTTP **provide route tables** and one server
  component mounts them. This is how the website, management API and REST ingest end up on one
  listener instead of the current seven, with the relationship stated rather than incidental.

### 3. Component lifecycle states

The correctness fix. A missing dependency must mean **waiting**, not refusal:

```
Waiting { on: Capability } → Running → Failed(err) → Backoff → Running
                                    ↘ Stopped
```

Specify the transitions, who drives them, backoff policy, and what a `Failed` component does to
components that depend on it (cascade, or let them keep their last good data?).

This directly replaces the current behaviour where four components refuse to construct without
TimescaleDB and stay dead until restart. It is also the surface W5 renders: the UI must be able to
show *which* capability a `Waiting` component is blocked on.

### 4. Map every existing component onto the model

Produce a table covering all eleven in the list above plus the station drivers and storage writers,
giving for each: trigger, requires, provides, and what its `tick` does. If anything doesn't fit, the
model is wrong — say so rather than bending the component.

Pay particular attention to:

- **Aeris**, which is two independent producers (forecasts, alerts) on separate schedules. Decide
  whether that's one component with two triggers or two components, and justify it.
- **Sun-times precompute**, which must stop living inside the REST server. It is nominally
  `OnStartup`, but a precompute has a finite horizon and a process that runs for months outlives it —
  decide the real trigger (`Calendar` daily extension, or `OnStartup` plus a scheduled top-up) rather
  than carrying the startup-only shape forward.
- **APRS**, which needs both `ReadingStore` and `StationRegistry`, and which also has two output
  transports (APRS-IS network, and KISS TNC over serial or TCP). Decide whether transport is a
  component concern or a sink concern.
- **The station drivers.** "Driver" is doing three jobs at once and must be decomposed — see §4a.

### 4a. Decompose "driver" into transport, codec and identity

A driver is not one thing. Measured across the seven existing drivers:

| Driver | Who initiates | Transport | Stations per endpoint |
|---|---|---|---|
| Davis | **we do** | serial *or* TCP | 1 |
| Campbell | **we do** | serial *or* TCP | 1 |
| Snow gauge | **we do** | gRPC client | 1 |
| AirGradient | **we do** | HTTP poll | 1 |
| Ambient | **they do** | HTTP server | 1 — *should be N* |
| WeatherLink Live | **both** | UDP listener **+** HTTP poll | 1 |
| gRPC receiver | **they do** | gRPC server | **N**, via a registry |

Two observations drive the design. **Connection direction and station cardinality are linked**: a
transport we initiate is inherently 1:1 (a serial port *is* one station), while one they initiate is
inherently 1:N, so station identity must come from the *data* rather than the config. And **Ambient
is paying for that not being modelled** — each device gets its own `http.Server` on its own port
(`Addr: listenAddr:s.config.Port`), with the station identified positionally as `s.config.Name`,
i.e. whichever server received the request. Two Ambient stations means two HTTP servers. The gRPC
receiver hit the same wall and solved it differently, by bolting a `RemoteStationRegistry` onto a
single device config — two mechanisms for one problem, neither of them the model.

Design the three-part split:

```
Transport ──frames──▶ Codec ──(StationKey, Reading)──▶ Identity ──▶ Envelope ──▶ stream
```

```rust
enum Transport {
    // we initiate — inherently one station
    Serial   { dev: PathBuf, baud: u32 },      // Trigger::Service
    TcpOut   { host: String, port: u16 },      // Trigger::Service
    HttpPoll { url: Url, every: Duration },    // Trigger::Every
    GrpcOut  { endpoint: Url },                // Trigger::Service

    // they initiate — inherently many stations, shared endpoint
    HttpIn   { mount: RoutePath },             // Trigger::Service, provides HttpRoutes
    UdpIn    { port: u16 },                    // Trigger::Service
    GrpcIn   { mount: RoutePath },             // Trigger::Service, provides HttpRoutes
}

enum StationIdentity {
    Fixed(StationId),               // serial / TCP / poll — config says which station
    FromPath,                       // /ingest/ambient/{station_key}
    FromQuery(&'static str),        // Ambient sends PASSKEY — identity and secret in one
    FromPayload(fn(&Frame) -> Key), // WeatherLink Live broadcasts its DSN in the packet
    FromRegistration,               // gRPC peers register and are issued an id
}
```

Codecs are **pure**: bytes or an HTTP request in, readings out. No I/O, so they are tested against
captured frames with no hardware.

Requirements for this section:

- Map all seven existing drivers onto `(Transport, Codec, StationIdentity)`. Any that doesn't fit
  means the model is wrong — say so rather than bending the driver.
- Show that Ambient becomes **N stations on one endpoint**, and that adding a second Ambient station
  is a config row rather than a second HTTP server.
- Show that WeatherLink Live is simply *two transports feeding one codec family*, rather than an
  anomaly.
- Show that `FromRegistration` is one identity strategy among five, so the remote-station registry
  generalises instead of being special-cased.
- **Identity resolution is the authentication point.** Today there effectively isn't one: Ambient
  trusts any request arriving on its port as being that station, and the gRPC receiver pushes a
  reading into the distributor *before* consulting the registry, only debug-logging an unknown
  sender. Specify where a shared secret or mTLS check belongs so it is one code path, not per-driver.
- Decide whether `HttpPoll` is genuinely a transport or just `Trigger::Every` wrapped around
  `TcpOut`. Either answer is defensible — make the call rather than leaving it implicit.
- Provide a convenience constructor for the common `Serial + Fixed` case. The abstraction should earn
  its keep on Ambient and WeatherLink Live without taxing Davis.

**Demonstrate the flexibility** by showing how each of these would be added, without new
infrastructure: an MQTT-subscribe station; a vendor cloud API poll (Tempest/WeatherFlow); a Modbus
sensor; and APRS-IS as an *input* — ingesting other stations' weather reports, which would be
`TcpOut` + an APRS weather codec + `FromPayload(callsign)`. The current architecture cannot express
that last one at all.

### 4b. Rate-limited outputs: the window is the hard part

APRS, Weather Underground, PWS Weather and Aeris can only transmit occasionally, while readings
arrive every few seconds. Rate limiting itself is **structural and needs no queue or buffer** — a
component with `Trigger::Every(interval)` simply does not wake more often than allowed. The real
problem is *what value to send*, because a point sample is usually the wrong number and the protocols
say so: CWOP wants wind direction and speed **averaged** with gust as a **peak over ~5–10 minutes**,
and Weather Underground's field names encode windows explicitly (`windspdmph_avg2m`,
`windgustmph_10m`).

The current code half-knows this and is inconsistent. One APRS packet draws from three sources: gust
from a dedicated `calculate_wind_gust()` SQL function (the right idea), wind direction and speed
straight off the reading (**wrong per CWOP — an instantaneous value where an average is required**),
and day-rain from `CalculateDailyRainfall` called inside the packet builder. The "latest reading" is
itself `weather_1m ... WHERE bucket > NOW() - INTERVAL '2 minutes' LIMIT 1` with **no `ORDER BY`**,
so it is an arbitrary bucket that may be two minutes stale — accidentally a 1-minute average.

Specify instead that an output encoder **declares the windowed values it needs**:

```rust
trait OutputEncoder {
    fn required(&self) -> &[(Metric, Window, Agg)];
    fn encode(&self, v: &WindowedView) -> Result<Vec<u8>>;
}
```

APRS then declares a table — `(WindDir, 5min, CircularMean)`, `(WindSpeed, 5min, Mean)`,
`(WindGust, 10min, Max)`, `(OutTemp, Instant, Last)`, `(RainDay, Day, Sum)` — reviewable line by line
against the CWOP specification instead of being smeared across a string builder.

The window summaries come from rolling up 1-minute aggregate buckets using each metric's declared
`agg` from the registry. This is the second payoff of storing composable aggregate state: because the
1m level holds `sum`/`count` rather than a finished `avg`, rolling ten buckets into a correct
ten-minute mean is arithmetic. The same `agg` field that generates the CAGG SQL tells the encoder what
to send.

### The APRS structural cleanup

APRS is the worst-shaped output and needs explicit instruction, because it is 991 lines across
`controller.go` (859) and `kiss.go` (132) while `pkg/aprs` — the library that should hold the
protocol — contains **two functions, 68 lines**, only passcode calculation.

- **Transport dispatch is a stringly-typed if/else with a silent fallback, and this is a live bug:**

  ```go
  if strings.ToLower(device.APRSTransport) == transportKISS {
      a.sendReadingViaKISS(...); return
  }
  a.sendReadingToAPRSIS(...)
  ```

  Two branches for three transports — KISS-serial versus KISS-TCP is decided much deeper, inside
  `openKISSConnection`. And an unrecognised transport string **silently transmits over APRS-IS**, so a
  typo of `"kis"` puts the weather report on the internet instead of the radio with no error. Replace
  with an exhaustive enum over `AprsIs`/`KissSerial`/`KissTcp` behind a `FrameSink` trait, so an
  unknown transport is a **config parse error**. The two KISS variants collapse into one
  implementation over a byte stream — the Go code half-discovered this already, since
  `openKISSConnection` returns an `io.ReadWriteCloser`.
- **Move the protocol encoding into `pkg/aprs`.** Twelve pure functions currently sit in the
  controller: `CreateCompleteWeatherReport`, `buildWeatherReportInfo`, lat/lon conversion,
  `AltitudeCompress`, `CourseCompress`, `SpeedCompress`, `LatPrecompress`, `LonPrecompress`,
  `EncodeBase91Position`, `EncodeBase91Telemetry`, and a hand-rolled `round()`. Pure, so they become
  golden-testable against captured packets — impossible today because the encoder is tangled with
  database access and transport.
- **One component instance per device**, not one controller managing N workers. This deletes
  `reconcileWorkers`, its `map[string]context.CancelFunc`, the private `sync.WaitGroup`, and the
  five-level `sendReports → reconcileWorkers → sendDeviceReports → sendReportForDevice →
  sendStationReading` chain. Per-device interval becomes that instance's trigger.
- **Delete the bespoke lifecycle and health monitor.** APRS invented `Start`, `Stop`, `IsRunning` and
  `GetHealth` because it needed them, but the `Controller` interface only declares
  `StartController()`, so `Stop()` is dead code. Component state is the health surface. The one useful
  part — can we authenticate to APRS-IS — becomes a `/test/*` check in the management UI alongside the
  other six.
- **No SQL in the radio component.** `sendStationReading` currently runs
  `SELECT calculate_wind_gust(?)` directly, with a comment saying *"same as website"* — one concept,
  two call sites, no owner. Wind gust is a derived value resolved by W3 §2a's rule.

Also specify:

- **Minimum intervals enforced at config parse time**, not per send. The floors exist
  (`MinPWSUploadInterval`, `MinWUUploadInterval` = 60s) but `ResolveUploadInterval` clamps and logs a
  warning on *every* call.
- **Retry must consume rate budget.** With `tower`, `ServiceBuilder` applies layers outside-in, so
  `Retry` goes **outside** `RateLimit` and each attempt takes its own permit. Reversed, every retry
  happens inside a single permit and blows through the limit on the first failure. State the ordering
  and why.
- **Adaptive pushback.** WU and PWS can return 429 with `Retry-After`; the limiter should honour that
  rather than running on a purely fixed schedule.
- **The radio asymmetry.** HTTP services can tell you to slow down. A radio cannot — APRS over KISS
  has no 429, and flooding a shared band has no feedback mechanism. So for the KISS transports the
  local floor is the only protection: make it a hard constraint in the type, not a config value with
  a warning. For APRS-IS and HTTP services, clamped-and-logged config is proportionate.

### 5. Interaction with the reading pipeline

`OnReading` is **one trigger among six**, not the organising principle of the system. State how the
stream is exposed as the `ReadingStream` capability, and how a component declaring `OnReading`
receives readings — including the policy for a component that can't keep up. W3 owns the pipeline's
internals; this document owns the boundary.

## Do not

- Write implementation code.
- Design the pipeline internals (W3), the metric registry (W2), or the UI (W5).
- Preserve the word "controller" or its `StartController()` shape.
- Invent capabilities that no existing component needs, beyond ones the collector work plainly
  requires.

## Done when

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`: **Decisions** (numbered one-liners), **Open questions** (with
  recommendations), **Assumptions about other work items**, and **Proposed amendments to
  `DESIGN.md`**. W6 consumes these; a document without them cannot be reconciled.

- The trait and both enums are specified, with the single-`tick` question resolved either way.
- All eleven existing components plus the station drivers and storage writers are mapped onto the
  model in a table, with any misfit called out.
- Capability resolution, startup ordering and cycle handling are specified.
- The lifecycle state machine is specified, including what `Failed` does to dependents.
- The `AppReloader` cycle is explicitly shown to be gone.
- `Calendar` handling of timezones, DST and missed windows is specified, and `Every` has a stated
  overlap policy and jitter story.
- §4a maps all seven drivers onto `(Transport, Codec, StationIdentity)`, shows Ambient as N-on-one-
  endpoint, and demonstrates the four new collection types.
- §4b specifies the `(Metric, Window, Agg)` declaration, the `Retry`-outside-`RateLimit` ordering,
  and the radio floor as a type-level constraint.
