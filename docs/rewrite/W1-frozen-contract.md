# W1 — Capture the frozen external API contract

**Depends on:** W0.
**Produces:** `docs/legacy-api-contract.md`, `contract/legacy-v1.json`, `contract/fixtures/`, and a
reusable conformance test.
**Writes no application code.** It produces a specification and a test corpus that later work items
must satisfy.

## Context

Read `docs/DESIGN.md` first.

Two independent clients must keep working after the rewrite, and **both** are frozen:

1. **The iOS app** — `git@github.com:chrissnell/RemoteWeather.app.git`. **This repo is currently not
   registered as a Multica project resource at all** (checked 2026-10-28), and platform agents can
   only check out registered repos — so it must be added before this work item runs, or the Swift
   side of deliverables 2 and 3 is impossible. Client code lives in
   `RemoteWeather/Sources/RemoteWeather/Data/REST/`.
2. **The existing public weather website**, which is being carried forward as-is rather than
   redesigned. Its JS lives in `internal/controllers/restserver/assets/js/`.

Server-side sources of truth: `internal/controllers/restserver/types.go` (the DTOs) and
`handlers.go` plus `controller.go` (routing) in the Go repo.

**Tie-break rule.** Each client's requirements are binding for the surface it uses, and the contract
is the **union** of both. Where a client is stricter than the server, the client wins — the server
may emit fields nobody reads, but it must never omit or null a field any client requires.

## Known surface

Starting point, not a substitute for your own enumeration.

| Endpoint | iOS | Website |
|---|---|---|
| `GET /latest` — called with `?station=<name>` **and** `?station_id=<int>` | yes | yes |
| `GET /span/{span}` — e.g. `/span/24h`, `?station=`, optional `&station_id=` | yes | yes |
| `GET /snow` | yes | yes |
| `GET /stationinfo` | yes | yes |
| `GET /forecast/{span}` — `?station_id=` | yes | yes |
| `GET /alerts` | — | yes |
| `GET /almanac` | — | yes |
| `GET /api/stations` | — | yes |
| `GET /api/remote-stations` | — | yes |
| `GET /`, `/portal`, `/privacy`, `/support` | — | yes (HTML) |
| `/js/`, `/css/`, `/fonts/`, `/images/` | — | yes (assets) |

Note that `/forecast/{hours}` and `/span/{hours}h` are **string-interpolated** in the Swift client,
so they do not appear in a naive search for path literals. Expect the same pattern in the website JS.

## Deliverables

### 1. Endpoint inventory

For each endpoint: exact path grammar, every query parameter any client sends, and the status codes
and error body shapes clients tolerate.

Search **all** iOS targets — app, watch, and widget extensions, plus the `Suncrest*` variants — not
just the main app target. Likewise cover every website JS module, including the portal and radar code.

### 2. Field-by-field response schema

Per endpoint: JSON key, type, whether the server currently emits it unconditionally or under
`omitempty`, and how each client decodes it.

Start from `restserver/types.go`, which has **133 JSON tags of which 31 are `omitempty`**, and the
Swift `WeatherResponse.swift`, which is 5 structs and ~126 decoded properties matching by member
name with a single explicit `CodingKey` (`schema_version`).

### 3. The compatibility hazard table

Compute the intersection of **{Go fields tagged `omitempty`}** and **{Swift properties declared
non-optional}**.

Every member of that set is a field that breaks iOS decoding whenever its value is zero, because a
non-optional Swift `Double` fails the *entire* decode when its key is absent. This is a live latent
bug in the shipping system.

**Record current behaviour. Do not fix it.** The new implementation must reproduce what shipped, not
what was intended. If a fix is warranted it is a separate, deliberate decision.

While you are here, note the quirks so nobody later "cleans them up":
- `ohum` and `outhumidity` both exist and carry the same measurement.
- Naming is inconsistent — `otemp`, `bar`, `winds`, `windd` run together, while `period_rain` is
  snake_case.
- `ts` is milliseconds since epoch as `Int64`. Confirm and pin this.

### 4. Golden fixtures

Captured real JSON responses, with the request that produced each recorded alongside:

- a station reporting a full sensor complement
- a station reporting a minimal one
- snow enabled, and snow disabled
- **snow device configured but physically absent or offline.** Capture it for `/snow` **and** for
  `/latest`, since snow-related fields appear in both. It matters because the iOS client decodes most
  fields as non-optional, so whatever shape the absent-sensor case produces is what the new
  implementation must reproduce. The owner confirms the snow hardware is currently down and **can be
  shut down on demand**, so this state is reproducible whenever you need it — no need to treat it as
  a one-time capture.
- a `/span/` response covering a gap in the data
- at least one empty/no-data case per endpoint
- for the website-only endpoints, at least one representative response each

### 5. The conformance test

A test that replays the fixtures against any HTTP base URL and asserts byte-level JSON
compatibility: key presence, types, and numeric formatting. Determine empirically whether Go's
encoder emits `0` or `0.0`, and whether either client's decoder is sensitive to field order.

It must be runnable against **both** the Go server and, later, the Rust one.

**It must pass green against the current Go server before this work item is complete.** That green
run is what proves the harness is trustworthy before anything depends on it.

### 6. HTML and asset surface

For `/`, `/portal`, `/privacy`, `/support` and the asset trees, the contract is weaker than JSON —
record what must hold (routes resolve, content types, cache headers, template variables consumed)
rather than attempting byte equality on rendered HTML. Flag that three Go templates exist
(`portal.html.tmpl`, `weather-station.html.tmpl`, `js/weather-app.js.tmpl`) and that **how** they
get served from Rust is W3's problem, not yours — just capture what they currently emit.

## Do not

- Modify either repo's source, or change any endpoint.
- Design the new v2 API. That is a later work item.
- Omit a field because it looks vestigial.
- "Improve" the contract in any way, including fixing the hazard from deliverable 3.

## Done when

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`: **Decisions** (numbered one-liners), **Open questions** (with
  recommendations), **Assumptions about other work items**, and **Proposed amendments to
  `DESIGN.md`**. W6 consumes these; a document without them cannot be reconciled.

- The conformance test passes green against the current Go server.
- The hazard table is complete and explicitly lists every affected field.
- Every endpoint in the table above is covered, plus any you discover.
- `contract/legacy-v1.json` is machine-readable and sufficient to drive the conformance test without
  reading the prose document.
