# W8 — CI gate and release pipeline

**Depends on:** W0 (repo decision). Can run in parallel with everything else.
**Produces:** working CI workflows, and `docs/release.md`.
**This item writes real configuration** — it is not design-only.

## Why this is its own work item

Every other prompt assumes a PR gate exists "from the first commit," and none of them designs it.
That is an unowned seam, and the Go repo demonstrates the cost: its **only** workflow is
release-on-tag. Nothing runs tests, vet, formatting or lint on a pull request. Measured consequences
in that tree — 45 files failing `gofmt`, two live data races that went unnoticed, and 0% test coverage
on `managers`, `storage`, `timescaledb`, `database`, `types` and six of seven station drivers.

This exists so the first implementation commit lands into a repo that can already tell it something
is wrong.

## Scope

### 1. The PR gate

Must be green before any implementation work item is considered done:

- `cargo build --all-targets`
- `cargo test --all` — including the concurrency tests W3 §8 specifies
- `cargo fmt --check`
- `cargo clippy -- -D warnings`
- **`cargo xtask codegen --check`** — fails if generated output has drifted from the metric registry
  (W2). This is the gate that keeps the registry authoritative rather than aspirational.
- the frozen-contract conformance suite from W1, replayed against the built binary

Decide what runs on every push versus only on PR, and what the required-checks set is.

### 2. Tests that need services

W3 §8 and W4 require a real TimescaleDB. Specify how CI provides it — a service container, and how
migrations are applied twice (from empty, and from the previous snapshot with seeded rows) with the
union-view assertions W4 defines.

### 3. Hardware-free driver tests

The four Go emulators stay in Go and are **fixed external test infrastructure** (see the capability
inventory). Specify how CI runs a Go emulator against a Rust driver — this is deliberate
cross-implementation validation, so an independently-written emulator is a stronger oracle than one
written alongside the driver.

### 4. The release pipeline

The Go `.goreleaser.yml` is 10.6 KB and produces the current target matrix. None of it survives, and
nothing else owns replacing it:

| Target | Notes |
|---|---|
| linux/amd64 | |
| linux/arm64 | Raspberry Pi |
| linux/armv7 | |
| linux/armv6 | the hard one |
| windows/amd64 | confirm whether still required |
| Docker image | multi-arch, currently via buildx + QEMU |
| deb / rpm / arch packages | plus systemd unit, sysusers, install script |

**The armv6/armv7 cross-compilation question is load-bearing and must be answered empirically.** The
Go build is painless specifically because `modernc.org/sqlite` is pure Go with no cgo. The question
is not hypothetical: `DESIGN.md`'s named-crates list already picks `sqlx` for both databases, and
sqlx's SQLite backend links `libsqlite3-sys` with bundled C exactly as `rusqlite` does — so the
chosen stack already has C in the armv6 path. Test this early — a cross-compile spike for armv6 —
because the answer may feed back into W2/W4's choice of SQLite driver (or into whether the config
store is SQLite at all). If it is bad, say so loudly rather than working around it.

Evaluate `cargo-dist` versus `cross` versus hand-rolled, and commit to one.

### 5. What the pipeline must not do

Do not sign, publish or push anything from CI without the owner's explicit configuration — the Go
pipeline publishes to ghcr.io and creates GitHub releases on tag, and that behaviour should be
re-established deliberately rather than inherited.

## Do not

- Design application architecture.
- Add coverage thresholds as a hard gate. Prefer a ratchet that cannot decrease, since a fixed target
  invites tests written to hit a number.
- Replicate the Go pipeline's structure. Replicate its *outputs*.

## Done when

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`.
- The PR gate exists, runs, and is green on an empty-but-building repo.
- The armv6 cross-compilation question has a measured answer, and any feedback into the SQLite driver
  choice is flagged as a proposed amendment.
- Every target in the matrix above is either produced or consciously dropped with a stated reason.
