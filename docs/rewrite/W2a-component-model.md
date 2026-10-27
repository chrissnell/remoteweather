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

### 2. Capabilities and the dependency graph

Define the capability set and the resolution rules. Starting point:

```rust
enum Capability {
    ReadingStream, ReadingStore, StationRegistry, ConfigStore,
    ForecastStore, AlertStore, SnowCache, SunTimes, HttpRoutes,
}
```

Specify:

- how `requires`/`provides` are resolved into a startup order, and what happens on a cycle
  (detected at startup and fatal, or detected at build time?)
- how a capability is handed to a component — typed handle, registry lookup, something else
- **how the `AppReloader` cycle is eliminated.** Reconfiguration should be a capability backed by
  the config watch channel, not a back-reference to the application object.
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
- **Sun-times precompute**, which is `OnStartup` and must stop living inside the REST server.
- **APRS**, which needs both `ReadingStore` and `StationRegistry`, and which also has two output
  transports (APRS-IS network, and KISS TNC over serial or TCP). Decide whether transport is a
  component concern or a sink concern.
- **The station drivers**, which are `Service`-triggered for listeners (Ambient's HTTP endpoint,
  WeatherLink UDP, gRPC receiver) but `Every`-triggered for pollers (AirGradient, WeatherLink HTTP)
  and `Service` for serial loops (Davis, Campbell). The same "driver" label covers three triggers.

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

- The trait and both enums are specified, with the single-`tick` question resolved either way.
- All eleven existing components plus the station drivers and storage writers are mapped onto the
  model in a table, with any misfit called out.
- Capability resolution, startup ordering and cycle handling are specified.
- The lifecycle state machine is specified, including what `Failed` does to dependents.
- The `AppReloader` cycle is explicitly shown to be gone.
- `Calendar` handling of timezones, DST and missed windows is specified.
