# W7 — The collector: multi-node ingest and forwarding

**Depends on:** W2a (component model), W3 (pipeline and HTTP layer), W2 (wire schema).
**Produces:** `docs/collector.md`, including the wire protocol and the spool schema.
**Writes no implementation code.**

## Why this is its own work item

A RemoteWeather node can act as a collector for other nodes. The owner's words: *"we didn't have a
clear model for a RW station acting as a collector for other RW stations."*

There is no model because the two halves borrowed the wrong abstractions:

- **Egress** — "send my readings to another node" — is implemented as a **storage engine**:
  `internal/storage/grpcstream` implements `StorageEngineInterface.StoreReading`. Being a source for
  someone else is modelled as *storing*.
- **Ingress** — "receive readings from other nodes" — is implemented as a **weather station**:
  `internal/weatherstations/grpcreceiver` implements `WeatherStation`. Being a sink is modelled as
  *a device*.

Neither is wrong by accident; each reached for the nearest existing interface. The consequence is that
the collector role has no name anywhere in the codebase, the two directions share no type, and a node
cannot describe its own position in a topology.

### Four measured defects

**1. Loop prevention is a name blocklist that doesn't generalise.** `sendReading` calls
`getGRPCReceiverDevices()` — a full `LoadConfig()` plus device scan — **on every single reading**, to
drop readings whose `StationName` matches a local `grpcreceiver` device name. But
`convertToInternalReading` preserves the *origin's* name, so the guard only fires when an incoming
station name coincides with a local receiver device's name. Name them differently and a mutual uplink
or any A→B→C→A cycle is not stopped. A config query per reading, enforcing a convention rather than an
invariant.

**2. No store-and-forward.** On send failure the reading is dropped:

```go
case reading := <-readingChan:
    if err := c.sendReading(reading); err != nil {
        c.logger.Errorf("failed to send reading: %v", err)
        go c.reconnect(ctx)      // and the reading is gone
    }
```

A satellite on a flaky link permanently loses everything measured while the stream is down. The wire
contract cannot fix this alone: `SendWeatherReadings(stream WeatherReading) returns (Empty)` is
client-streaming with a single terminal `Empty` — no per-reading acks, no sequence numbers, no resume
offset. Even with a buffer the satellite could not learn what the collector persisted. That
`go c.reconnect(ctx)` also spawns an unbounded reconnect goroutine per failed send.

**3. Registration doubles as a credential handoff.** `RemoteStationConfig` carries `aprs_password`,
`wu_api_key`, `aeris_client_secret` and `pws_password`, and the collector's registry turns them into
its own uploader configs. The *intent* is sound — the collector is the fleet's egress point for
third-party services — but "here is who I am" and "here are my secrets, upload on my behalf" are two
operations welded together, so every satellite ships plaintext credentials over the wire at
registration and they come to rest in the collector's config database.

**4. No admission control in the data path.** In `SendWeatherReadings` the reading goes to
`s.ReadingDistributor` *first*; the `registry.GetByID(reading.StationId)` check happens afterward and
only debug-logs an unknown station. An unregistered sender's readings are stored anyway, under
whatever station name they claim.

## Scope

### 1. The role model

Per W2a, ingest and egress are **components with transports**, not storage engines and weather
stations. A collector is simply a node whose components include an ingest endpoint; a satellite is one
whose components include an uplink. Same binary, same code path, different configuration — so "both a
source of its own readings and a sink for others" is the default case rather than a special mode.

Specify how this composes with W2a §4a's `Transport`/`Codec`/`StationIdentity` split, noting that
`GrpcIn`/`HttpIn` with `FromRegistration` is exactly the ingest side and needs no new machinery.

### 2. Provenance and loop prevention as an invariant

```rust
struct Envelope {
    reading:     Reading,
    origin:      StationId,             // who physically measured it
    path:        SmallVec<[NodeId; 4]>, // nodes it has traversed
    seq:         u64,                   // per-origin monotonic
    received_at: DateTime<Utc>,
}
```

An uplink refuses to forward an envelope whose `path` already contains the peer's `NodeId`. Specify
that this is checked in the forwarding component with no config lookup, and that it therefore supports
arbitrary topologies — **multi-hop relay becomes expressible**, where today it is either unsafe or
accidentally forbidden depending on how things were named.

### 3. Delivery semantics — the part that must change on the wire

Make the stream bidirectional and acked:

```proto
rpc Ingest(stream IngestBatch) returns (stream IngestAck);

message IngestBatch { repeated Envelope envelopes = 1; }
message IngestAck   { string origin = 1; uint64 through_seq = 2; }  // cumulative
```

Specify:

- **The satellite's durable spool**: schema, where it lives, appended before the send, truncated only
  on ack, resumed from the last acked `seq` on reconnect. Bounded, with a stated policy for what
  happens when the bound is reached.
- **Idempotent write on the collector**: `UNIQUE (origin, seq)` with `ON CONFLICT DO NOTHING`, so
  at-least-once delivery plus an idempotent write gives effectively-once.
- **Observability**: "how far behind is each satellite" must be a number — spool depth and ack lag per
  peer, surfaced for W5. Today it is invisible.
- **REST parity.** If REST ingest is offered alongside gRPC (W3 treats transport as a codec choice), it
  needs the *same* contract — `POST /v1/ingest` returning `{origin, through_seq}` — or you get two
  reliability tiers and the model is unclear again. Same spool, same idempotent write.

### 4. Identity, registration and admission

- **Registration carries identity only.** `Hello(NodeIdentity) returns (HelloAck)` — node id, station
  metadata, no secrets.
- **Credentials stay where they are configured.** The collector uploads on behalf of the fleet by
  selecting origins, not by receiving secrets: an uploader component declares which origins it covers
  (e.g. `for = ["suncrest", "satellite-a"]`) and uses the collector's own credentials. This removes
  defect 3 entirely.
- **Admission happens before the pipeline.** mTLS or a per-peer pre-shared secret, checked before an
  envelope reaches the reading stream. Specify what happens to an unknown peer — rejected, not stored
  with a debug log.

### 5. Configuration shape

Make the topology declarative and symmetric, so a node's position is legible from its config alone.
Show a collector, a satellite, and a node that is both.

## Do not

- Redesign the component model (W2a) or the pipeline internals (W3).
- Design the metric registry or the wire schema's *field* layout (W2) — this item owns the
  *envelope and the RPCs*, not the reading's fields.
- Preserve the storage-engine or weather-station framing for either direction.
- Write implementation code.

## Done when

- The document ends with the four required sections from the output contract in
  `docs/rewrite/README.md`: **Decisions**, **Open questions**, **Assumptions about other work items**,
  and **Proposed amendments to `DESIGN.md`**.
- All four measured defects above are addressed, each with the mechanism that makes the defect
  unrepresentable rather than merely fixed.
- The spool schema, the ack/resume state machine, and the idempotency constraint are specified.
- Multi-hop relay is shown to work, with the loop invariant stated.
- Credential handoff is gone, and the replacement is specified.
