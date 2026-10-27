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

## 0. Vocabulary — the user-facing language is a deliverable

The owner's feedback, verbatim: *"it's not obvious what a 'controller' is, or a 'storage backend'."*
That is correct, and fixing it is a design task, not a copy-editing pass.

**There is a specific hazard to avoid here, created by the new architecture.** W2a retires the word
"controller" internally and replaces it with `Component`, `Capability`, `Trigger`, `Source` and
`Sink`. Those names are *better for the code and worse for the user.* "Component APRS is Waiting on
Capability ReadingStore" is less comprehensible than what exists today, not more. So:

> **The internal model's vocabulary must not reach the screen.** Produce an explicit translation
> table between internal names and user-facing language, and treat any internal term appearing in the
> UI as a bug.

Starting point — refine it, but the direction is non-negotiable:

| Internal / current | On screen |
|---|---|
| Controller | *never shown* — split by what it actually does, below |
| `restserver` controller | **Website & API** |
| `management` controller | **Admin** (it is the thing you are looking at) |
| `pwsweather`, `wunderground`, `aerisweather`, `aprs` controllers | **Weather Services** — "share my data" |
| Storage backend | **Data storage** — "where your readings are kept" |
| `timescaledb` storage | **Weather database** |
| `grpcstream` storage | **Send my readings to another RemoteWeather** |
| `grpcreceiver` device | **Receive readings from another RemoteWeather** |
| Device | **Weather station** — used consistently; today it alternates with "device" |
| Component / Capability / Trigger / Sink / Source | *never shown* |
| Snow cache | **Snow calculations** |
| Aeris | **Forecasts & alerts** (vendor named as a detail, not as the feature) |

**Organise the navigation by user intent, not by internal architecture.** A new user does not think
"I need to configure a controller," they think "I want my data on Weather Underground." Proposed
top level, to be refined:

- **My weather stations** — the hardware
- **Share my data** — Weather Underground, PWS, APRS/CWOP, forecasts
- **My website** — the public site and its settings
- **Data storage** — the weather database, and forwarding to another RemoteWeather
- **System** — logs, diagnostics, updates, admin account

**Status and error text is where jargon does the most damage**, because that is exactly when a
confused user is reading. Specify the user-facing rendering of every lifecycle state from W2a §3:

| Internal | On screen |
|---|---|
| `Waiting { on: ReadingStore }` | "Can't start yet — waiting for the weather database to connect." |
| `Failed(e)` | "Stopped because of an error: *<plain description>*." plus the detail behind a disclosure |
| `Backoff { until }` | "Retrying in 30 seconds." |
| `Trigger::Every(600s)` | "Uploads every 10 minutes." |

**Every label needs a one-line explanation available in place.** Not a separate manual — inline help
text or a hover, written for someone who has just bought a weather station, not for someone who has
read this design document.

**First-run is the most important screen in the application.** A new user with no configuration
currently meets a blank UI with tabs labelled "Controllers" and "Storage," which explains nothing.
Design a guided path: connect a weather station → confirm readings are arriving → optionally set up a
website → optionally share to weather services. This subsumes the "no stations configured" empty
state in §8 and should be treated as the primary onboarding flow rather than an edge case.

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

## 3. Authentication — username and password

**Owner decision:** a username and password. **No RBAC, and no multiple accounts for now** — design
for exactly one administrator, and do not build a user table, roles or permissions. It must also have
a defined process for setting the password on initial startup.

### What is being replaced

- a UUID used as a bearer token
- compared with `==`, i.e. not constant-time
- logged in cleartext at startup **and** on every failed attempt (`"Bearer token mismatch: got %s,
  expected %s"`), including into the in-memory buffer this very UI displays
- session cookie set to the token value itself, so a browser session cannot be revoked without
  rotating the API credential
- `Access-Control-Allow-Origin: *` on the admin API
- default bind `0.0.0.0`

### Requirements

**Credential storage.** Password hashed with **Argon2id**, storing the full PHC string so the
parameters travel with the hash and can be upgraded later — verify, and if the stored parameters are
below current policy, re-hash transparently on successful login. Never store the password, never log
it, never return it from any endpoint.

**Sessions are not credentials.** Login exchanges username and password for a **session**: a fresh
random token, stored server-side as a hash, delivered as an `HttpOnly` `SameSite=Strict` cookie with
`Secure` when served over TLS, with an expiry. Sessions are individually revocable, plus a "sign out
everywhere." This is the specific thing that is broken today, where the cookie *is* the long-lived API
credential and a browser session cannot be revoked without rotating it.

**Decide whether API tokens are needed at all.** The iOS app uses the *public* API, which is
unauthenticated, so nothing currently requires a programmatic credential for the admin API. Recommend
either omitting API tokens from v1 or providing exactly one explicitly-created, shown-once, revocable
token for scripting. Do not reintroduce "the session cookie is the API token."

### Initial password setup

Design this properly; it is the flow most likely to be got wrong.

**Primary path — one-time setup claim.** On first start with no password configured, generate a
single-use setup token, print it **once** to stderr/console clearly marked as one-time, and have the
UI present a "create your administrator account" screen that requires it. The token is invalidated the
moment an account exists.

**The hazard to design against explicitly:** an unclaimed instance bound to `0.0.0.0` means *whoever
reaches the setup page first owns the install.* That is a real vulnerability in naive first-run flows.
Requiring the setup token closes it, because only someone with console or log access has the token.
State this reasoning in the design so it is not "simplified away" later. Consider also binding to
loopback until claimed, and say whether that is worth the friction for remote installs.

**Recovery path — a CLI subcommand.** Something like `remoteweather reset-password`, runnable on the
host. The trust boundary is local filesystem access, which is the right one: if you can read the
config database you can already read everything. This is the locked-out answer, and it means the
system never needs a default or backdoor credential.

**Automation path — optional.** An environment variable or config entry for the initial password, for
Docker and provisioning. If offered, note the tradeoff plainly: it ends up in shell history, compose
files and the process environment.

**Never ship a default password**, and never allow an empty one.

### Proportionate hardening

Single operator on a trusted network, so keep it modest, but include:

- rate limiting or progressive delay on failed logins — trivial to add, and the only protection
  against a brute force
- minimum password length (12 is reasonable) and **no composition rules**, per current NIST guidance,
  which deprecates mandatory character-class requirements
- changing the password requires the current password
- constant-time comparison anywhere a secret is compared
- nothing secret in logs — the one-time setup token is the single exception, printed once by design
- replace wildcard CORS with an allowlist, and reconsider the default bind address

Note for the record that none of this touches the **public** API, which is unauthenticated and frozen
by W1 — so there is no contract risk here.

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

## 6. Technology — decided: Svelte, with the owner's `chonky` toolkit

**This is settled; do not re-evaluate it.** Owner decision: **Svelte**, which they use elsewhere, and
**their own UI toolkit `chonky`** — <https://github.com/chrissnell/chonky>.

Your job is not to choose a stack, it is to use this one well:

- **Read `chonky` first** and inventory what it actually provides — components, layout primitives,
  theming, form handling, conventions. Build the design on top of it rather than alongside it. If a
  screen needs something `chonky` does not have, say so explicitly and propose either an addition to
  `chonky` or a local component, with a reason; do not silently introduce a second component
  vocabulary.
- **Map every screen to `chonky` components** where they exist. Where the toolkit's conventions
  conflict with something proposed in this document, the toolkit generally wins — it is the thing the
  owner maintains and reuses across projects.
- **Note its gaps as findings**, since they are useful to the owner beyond this project.
- The generated OpenAPI client from §4 still applies: typed client, drift as a build failure.

Commit to a **bundle-size budget** and a **build step**. The current UI has neither, ships unminified,
and vendors a deprecated copy of moment.js. Svelte compiles away most of the framework, so the budget
should be genuinely small — state a number and what counts toward it.

The earlier open question about Rust-in-the-browser (Leptos/Dioxus) is **closed**. Record it as
considered and rejected in favour of an existing, maintained, owner-authored toolkit; that is a
stronger reason than any type-sharing argument.

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

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`: **Decisions** (numbered one-liners), **Open questions** (with
  recommendations), **Assumptions about other work items**, and **Proposed amendments to
  `DESIGN.md`**. W6 consumes these; a document without them cannot be reconciled.

- All 46 existing routes are mapped, consciously dropped with a reason, or marked not-UI-facing.
- The configured-versus-running model is specified, including the API it requires.
- A user-facing vocabulary is specified with an internal→on-screen translation table, and no internal
  term from W2a (`Component`, `Capability`, `Trigger`, `Source`, `Sink`) appears in any proposed
  screen, label or status message.
- Navigation is organised by user intent, and every one of W2a's lifecycle states has a plain-language
  rendering.
- The first-run guided setup flow is designed.
- The auth model is specified: Argon2id storage, sessions distinct from credentials, the one-time
  setup-claim flow with its "first to the page wins" hazard addressed, and a CLI recovery path.
- The design is built on `chonky` components, its gaps are listed as findings, and a bundle budget and build tool are named.
- Every error and empty state in section 8 has a specified behaviour.
