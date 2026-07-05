# APRS KISS TNC Output — Design

## Problem

The APRS controller currently transmits weather packets only to APRS-IS over
TCP (`internal/controllers/aprs/controller.go`). There is no way to send the
same packets over the air through a locally attached or networked TNC. We want
to add KISS output so a station can key up an RF transmitter via a TNC, using
either a **serial** connection (e.g. `/dev/ttyUSB0`) or a **network** connection
(e.g. Direwolf's KISS TCP port, `127.0.0.1:8001`).

## Goal

Send the existing APRS weather report over KISS to a remote TNC. Support both
serial and network connections and make both selectable per device. The APRS-IS
path stays the default and is unchanged.

## Non-goals

- Receiving from the TNC (this is TX-only, matching current APRS behavior).
- Relaying KISS config to remote stations over gRPC — KISS is a local transmit
  concern, so it lives only in the local (SQLite) device config.
- A brand-new top-level controller. KISS is an alternate **transport** for the
  same report, so it belongs inside the APRS controller as an option, not a
  parallel controller. This keeps report generation single-sourced.

## Configuration model

APRS config is already per-device (`DeviceData`), so KISS is added there as an
alternate transport rather than a separate `enabled` flag:

| Field (JSON / column)         | Type    | Default             | Meaning |
|-------------------------------|---------|---------------------|---------|
| `aprs_transport`              | TEXT    | `aprs-is`           | `aprs-is` or `kiss` |
| `aprs_kiss_connection`        | TEXT    | (empty)             | `serial` or `tcp` (required when transport=kiss) |
| `aprs_kiss_serial_device`     | TEXT    | (empty)             | e.g. `/dev/ttyUSB0` |
| `aprs_kiss_serial_baud`       | INTEGER | 9600                | serial baud rate |
| `aprs_kiss_tcp_address`       | TEXT    | (empty)             | `host:port` of the network TNC |
| `aprs_kiss_path`              | TEXT    | `WIDE1-1,WIDE2-1`   | AX.25 digipeater path (comma-separated) |
| `aprs_kiss_destination`       | TEXT    | `APRS`              | AX.25 destination tocall |

Rationale:
- `aprs_transport` selects the path cleanly without introducing a second enable
  boolean that could conflict with `aprs_enabled`. A device is APRS-enabled as
  today; `aprs_transport` decides where the packet goes.
- The existing `APRSCallsign` is reused as the AX.25 **source** address and
  supports the standard `CALL-SSID` form (e.g. `W1AW-13`).
- APRS-IS passcode is irrelevant to KISS/RF (RF requires a licensed callsign and
  has no passcode), so no passcode field is involved on the KISS path.

## New package: `pkg/kiss`

Pure, I/O-free encoders so the byte-level logic is fully unit-testable:

- `EncodeAX25UI(source, dest string, path []string, info []byte) ([]byte, error)`
  builds an AX.25 UI frame:
  - Address fields in order: destination, source, then each digipeater. Each is
    6 characters (space-padded) left-shifted one bit, followed by an SSID byte.
    The final address field has its low bit set (the HDLC address extension
    bit); all others clear.
  - Control byte `0x03` (UI frame), PID `0xF0` (no layer-3 protocol).
  - Info bytes appended verbatim.
  - Parses `CALL-SSID`, validates callsign length (≤6) and SSID (0–15).
- `EncodeFrame(frame []byte) []byte` wraps a raw frame in KISS:
  - `FEND (0xC0)`, command/port byte `0x00` (data frame, port 0), the payload
    with `FEND`→`FESC TFEND` and `FESC`→`FESC TFESC` escaping, then `FEND`.

## Refactor: single-source the report payload

`CreateCompleteWeatherReport` currently emits a full TNC2 string
(`CALL>APRS,TCPIP:<info>`). Split the info generation out:

- `buildWeatherReportInfo(device, reading, windGust, symTable, symCode) string`
  returns just the info payload (the part starting with `!...`).
- APRS-IS path: `CALLSIGN>APRS,TCPIP:` + info (byte-identical to today).
- KISS path: `EncodeAX25UI(callsign, destination, path, []byte(info))` then
  `EncodeFrame(...)`.

This guarantees both transports emit the same weather data.

## KISS transport

New file `internal/controllers/aprs/kiss.go`:

- `sendKISSFrame(ctx, device, frame []byte) error` opens the configured
  connection, writes the KISS-framed bytes, and closes it.
  - `serial`: `serial.OpenPort(&serial.Config{Name: ..., Baud: ...})` using
    `github.com/tarm/goserial`, matching the Davis station pattern.
  - `tcp`: `net.Dialer{Timeout: 3s}.DialContext(ctx, "tcp", addr)`.
  - Connections are opened per report (mirrors the current APRS-IS per-report
    dial). At a 5-minute cadence there is no need for a persistent connection or
    reconnect loop.

## Dispatch

Rename `sendStationReadingToAPRSIS` → `sendStationReading` and branch on
`device.APRSTransport`:
- `""`/`aprs-is`: existing APRS-IS flow, unchanged.
- `kiss`: build info → AX.25 UI frame → KISS frame → `sendKISSFrame`.

## Health monitor

The APRS-IS login test is meaningless for KISS devices. `updateHealthStatus`
counts only APRS-IS-transport devices for the login test; KISS devices are
reported healthy when their config validates (serial device or TCP address
present for the chosen connection type). Optionally, for `tcp` KISS a quick dial
check could verify reachability, but config validation is sufficient for v1.

## Config plumbing

Wire the new fields through:
- `pkg/config/provider.go` — `DeviceData` struct fields.
- `pkg/config/provider_sqlite.go` — embedded `CREATE TABLE` schema, `GetDevices`
  and `GetDevice` SELECT column lists + scan, `AddDevice` INSERT, `UpdateDevice`
  UPDATE.
- `migrations/config/026_add_aprs_kiss_fields.{up,down}.sql`.
- Management UI (`assets/index.html`, `assets/js/management-weather-stations.js`)
  — a transport selector plus KISS connection subfields shown when
  transport = kiss.

## Testing

- `pkg/kiss` unit tests: AX.25 address encoding for a known callsign+SSID, path
  handling, control/PID bytes, and KISS `FEND`/`FESC` escaping round-trips
  against known-good byte sequences.
- Report-info test: `buildWeatherReportInfo` output equals the substring after
  `:` in the current `CreateCompleteWeatherReport` output for the same inputs.

## Wiki

Update the wiki with a short "APRS KISS output" note pointing at `pkg/kiss`,
`internal/controllers/aprs/kiss.go`, and the config fields so future agents find
the code quickly.
