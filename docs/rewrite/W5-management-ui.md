# W5 — Management UI design

**Depends on:** W3 (needs the supervision/running-state model) and W2 (needs the OpenAPI surface).
**Produces:** `docs/management-ui-design.md`.
**Writes no implementation code.** Wireframes as ASCII or described layouts are welcome.

## Context

Read `docs/DESIGN.md` and `docs/service-architecture.md` first.

**The public weather website is out of scope and frozen.** It is carried forward unchanged, and how
it's served is W3's section 6. This work item covers only the operator/admin interface.

The existing Go management UI is the functional reference: `internal/controllers/management/`
(handlers, **46 HTTP routes**) and `assets/js/` (12 modules, ~7,000 lines, of which `management.js`
alone is 2,819, with no build step). Take its *feature set* seriously and its *implementation* as a
cautionary tale.

---

## 1. Functional inventory

Derive it from the 46 existing routes and organise **by operator task, not by endpoint**. Ground to
cover:

- weather-station CRUD, including the device-level enable/disable toggle
- storage backend configuration and health
- sink/controller configuration
- website configuration, including radar registration
- live log viewing
- system info and serial-port enumeration
- WeatherLink Live discovery, template suggestion, and template validation
- the remote-station registry
- auth and token rotation
- the `/test/*` family — device, serial port, database, storage, API, current-reading connectivity
  checks. This is ~780 lines of genuinely useful diagnostics in the Go version and it should survive.

Every route must end up either mapped to a screen, consciously dropped with a stated reason, or
marked as not-UI-facing.

## 2. Configured versus running — the distinction that doesn't exist today

The most valuable thing this UI can gain.

In the new architecture config is distributed over a watch channel and sources/sinks are supervised
tasks, so the UI can show **both** desired state and actual state: which stations are running, which
sinks are connected, spool depth, last successful delivery, last error per component.

Reference failure: in the Go version, editing a controller's settings appears to succeed and does
nothing until a restart, because reload only diffs which controller *types* are present and never
compares their contents. `ReloadStorageConfig` additionally tears down the gRPC-stream engine on
every reload because it's omitted from the should-be-active set.

**Design so that class of lie is impossible.** Every config change must surface whether it actually
took effect. Specify the API shape this needs — it will be new, additive endpoints.

**Show the dependency graph, not just per-component status.** Per W2a each component declares the
capabilities it requires and provides, and a component blocked on an unavailable dependency sits in
`Waiting { on: Capability }` rather than failing. The operator needs to see *which* capability is
blocking — "APRS is waiting on ReadingStore" is actionable; "APRS is not running" is not. Design for:

- per-component state: `Waiting` (on what), `Running`, `Failed` (with the error), `Backoff` (until when)
- for scheduled components, last run, last outcome, and next scheduled run — the supervisor owns the
  timers, so this is real state rather than a guess
- a view of what a failing capability is blocking downstream, since one unhealthy database puts
  several components in `Waiting` at once and the operator should see one root cause, not five
  unexplained outages

## 3. Authentication

Specify a replacement for the current scheme, which is:

- a UUID used as a bearer token
- compared with `==`, i.e. not constant-time
- logged in cleartext at startup **and** on every failed attempt (`"Bearer token mismatch: got %s,
  expected %s"`), including into the in-memory buffer this very UI displays
- session cookie set to the token value itself, so a browser session cannot be revoked without
  rotating the API credential
- `Access-Control-Allow-Origin: *` on the admin API
- default bind `0.0.0.0`

Decide the token/session model, the rotation flow, transport requirements, and what the UI shows
about active sessions.

Keep it **proportionate** — single operator, trusted network, not a multi-tenant SaaS. But no worse
than: a correctly-compared secret, sessions distinct from API credentials, and nothing secret in
logs.

## 4. API contract

The UI consumes a **generated** client from the OpenAPI schema produced by W2 and the service
crates. Specify:

- how the generated client is wired in and regenerated
- how drift between UI and server becomes a **build failure** rather than a runtime 404
- the endpoints this UI needs that don't exist yet — notably the running-state surface from section
  2 — designed as additive, versioned routes

Do not block on W2's artifacts existing; design against the schema's shape.

## 5. Live data

Logs and status stream over **SSE**, consistent with the service design. The current UI polls.

Specify: reconnect behaviour, backfill-on-connect from the ring buffer that W3 defines, client-side
filtering by level/module/station, and what the operator sees when the buffer overflows.

Note that W3 specifies runtime-adjustable log levels via `tracing_subscriber::reload` — so
"raise `weatherstations::davis` to DEBUG for ten minutes" is a UI feature worth designing.

## 6. Technology choice, with an explicit budget

Recommend a stack and justify it against the alternatives, **including** the option of
Rust-in-the-browser (Leptos/Dioxus) versus TypeScript with types generated from the same registry.

Weigh honestly: this is a low-traffic, single-operator, form-heavy, correctness-critical UI, and the
person maintaining it values clarity over novelty. Rust end-to-end buys type sharing for unit
conversion and validation; the ecosystem for forms and tables is thinner, and this UI is mostly
forms and tables.

Commit to a **bundle-size budget** and a **build step**. The current UI has neither, ships
unminified, and vendors a deprecated copy of moment.js.

## 7. Information architecture

Navigation, page inventory, and for each page: what it shows, what can be edited, and what
validation happens client-side versus server-side.

Call out where the current UI's structure actively misleads — in particular anywhere it presents
configuration as applied when it isn't.

## 8. Error and empty states

Specify what the operator sees for:

- no stations configured yet (first-run)
- a station configured but failing to connect
- storage unreachable
- a sink backed up with a deep spool
- a validation failure on save
- a config change that was accepted but has not yet taken effect

These are most of the real screen time for an admin UI, and the usual place design effort doesn't go.

---

## Do not

- Touch the public weather website.
- Design the metric registry (W2) or the service pipeline (W3).
- Write implementation code.
- Expand into dashboards and charting. This is a configuration and operations tool; the weather
  display already exists and is frozen.

## Done when

- All 46 existing routes are mapped, consciously dropped with a reason, or marked not-UI-facing.
- The configured-versus-running model is specified, including the API it requires.
- The auth model is specified.
- The technology recommendation names a bundle budget and a build tool.
- Every error and empty state in section 8 has a specified behaviour.
