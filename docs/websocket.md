# WebSocket Protocol

Symphonia's real-time call protocol is version 1 and is served at:

```text
GET /api/calls/{code}/ws
```

## Connection and authentication

The browser must request both WebSocket subprotocols:

```text
symphonia.v1, auth.<JWT>
```

The JWT must never be placed in the URL. The server validates it before upgrading, verifies that its user is an active participant of a non-ended call, and negotiates only `symphonia.v1`. Missing or invalid credentials, inactive participants, ended calls, and disallowed origins are rejected as HTTP responses before the upgrade.

Origins must either exactly match the backend request origin or be listed in the comma-separated `WS_ALLOWED_ORIGINS` environment variable. Its development default is `http://localhost:5173,http://127.0.0.1:5173`.

## Envelopes

Every client text frame has this shape:

```json
{
  "version": 1,
  "type": "message",
  "request_id": "request-42",
  "data": {"text": "Hello"}
}
```

`request_id` is required, nonempty, and at most 64 bytes. `data` must be a JSON object. Every server text frame has this shape:

```json
{
  "version": 1,
  "type": "message",
  "request_id": "request-42",
  "data": {},
  "occurred_at": "2026-09-16T12:34:56.123456Z"
}
```

`request_id` is included on direct responses and errors when the client request ID can be recovered. Broadcasts omit it. `occurred_at` is an RFC 3339 timestamp.

## Client events

The first frame after upgrade must be `join_call` with empty data:

```json
{"version":1,"type":"join_call","request_id":"join-1","data":{}}
```

The caller receives the current authoritative call and ephemeral mute states:

```json
{
  "version": 1,
  "type": "join_call",
  "request_id": "join-1",
  "data": {
    "call": {"id":10,"code":"room1234","host_user_id":1,"status":"active","participants":[],"created_at":"2026-09-16T12:00:00Z","updated_at":"2026-09-16T12:00:00Z","ended_at":null},
    "mute_states": [{"user_id":2,"muted":true}]
  },
  "occurred_at": "2026-09-16T12:34:56Z"
}
```

The other connected participant receives the following event. A successful REST join publishes the same notification, and reconnecting a socket publishes it again.

```json
{"version":1,"type":"participant_joined","data":{"participant":{"user_id":1,"name":"Ada","spoken_language":"EN-US","heard_language":"PT-BR","joined_at":"2026-09-16T12:00:00Z"}},"occurred_at":"2026-09-16T12:34:56Z"}
```

Send a transient text message with:

```json
{"version":1,"type":"message","request_id":"message-1","data":{"text":"Hello"}}
```

Text is trimmed and must contain 1 to 1000 Unicode codepoints. The message is broadcast to both participants, including its sender:

```json
{"version":1,"type":"message","data":{"id":"8aceebd33d7a4e9291ecb3909c015cfe","user_id":1,"name":"Ada","language":"EN-US","text":"Hello","sent_at":"2026-09-16T12:34:56Z"},"occurred_at":"2026-09-16T12:34:56Z"}
```

Send a mute state with:

```json
{"version":1,"type":"mute_state","request_id":"mute-1","data":{"muted":true}}
```

It is broadcast to both participants, including its sender:

```json
{"version":1,"type":"mute_state","data":{"user_id":1,"muted":true},"occurred_at":"2026-09-16T12:34:56Z"}
```

Only `join_call`, `message`, and `mute_state` are accepted from clients. Unknown types, server-only types, unsupported versions, malformed envelopes, and invalid data produce an `error` event without exposing internal details:

```json
{"version":1,"type":"error","request_id":"message-1","data":{"code":"invalid_data","message":"text must contain 1 to 1000 Unicode codepoints"},"occurred_at":"2026-09-16T12:34:56Z"}
```

## Binary audio

After `join_call` succeeds, either participant may send PCM audio as a WebSocket binary frame. Audio is not a JSON event and has no JSON envelope. Every frame is exactly 664 bytes:

| Offset | Size | Encoding | Value |
| --- | ---: | --- | --- |
| 0 | 2 | ASCII | Magic `SA` (`0x53 0x41`) |
| 2 | 1 | unsigned byte | Version `1` |
| 3 | 1 | unsigned byte | Kind `1` (PCM audio) |
| 4 | 1 | bit flags | Bit 0 permits discontinuity; all other bits must be zero |
| 5 | 1 | unsigned byte | Codec `1` (signed PCM16LE) |
| 6 | 1 | unsigned byte | Channels `1` (mono) |
| 7 | 1 | unsigned byte | Header size `24` |
| 8 | 4 | uint32, big-endian | Nonzero stream ID |
| 12 | 4 | uint32, big-endian | Sequence number |
| 16 | 4 | uint32, big-endian | Sample timestamp |
| 20 | 2 | uint16, big-endian | Sample rate `16000` Hz |
| 22 | 2 | uint16, big-endian | Sample count `320` |
| 24 | 640 | 320 signed int16, little-endian | PCM samples |

The sequence and sample timestamp fields may use the full uint32 range. A frame represents 20 ms of mono audio. The server validates every field. When translation is unnecessary it relays frames unchanged to the other ready participant. Otherwise it sends microphone PCM to the translation service and returns translated speech using the same binary format, resampled to 16 kHz with a new stream ID. It never echoes audio to the sender. Audio sent while muted or without a ready peer is dropped.

An invalid binary frame is dropped and produces the generic JSON error `invalid_audio_frame`; validation details are intentionally not exposed. The first frame after upgrade must still be the text `join_call` event. Binary data before it produces an error and an unsupported-data close.

Audio delivery is bounded and best effort. Slow or failed peer writes are timed out after approximately one second and the failed peer is removed. Frames are not queued for replay or persisted. When the speaker's language already matches the peer's heard language, frames are relayed byte-for-byte. Otherwise the PCM payload is streamed to the configured translation provider and the peer receives `input_transcription`, `output_transcription`, `translated_audio`, and, when applicable, `translation_interrupted` JSON events. See [translation.md](translation.md).

## Server notifications

Successful REST operations are authoritative and publish their resulting state to sockets in the room.

After a successful language update:

```json
{"version":1,"type":"language_changed","data":{"participant":{"user_id":1,"name":"Ada","spoken_language":"PT-BR","heard_language":"EN-US","joined_at":"2026-09-16T12:00:00Z"}},"occurred_at":"2026-09-16T12:34:56Z"}
```

After a successful REST leave, remaining participants receive `reason: "left"` and the leaving user's socket is closed:

```json
{"version":1,"type":"participant_left","data":{"user_id":1,"reason":"left"},"occurred_at":"2026-09-16T12:34:56Z"}
```

An ordinary socket disconnect removes only the in-memory connection and emits `reason: "disconnected"`; it does not alter database membership, so the user can reconnect:

```json
{"version":1,"type":"participant_left","data":{"user_id":1,"reason":"disconnected"},"occurred_at":"2026-09-16T12:34:56Z"}
```

A reconnect emits `participant_joined` again. There is at most one socket per user in each call. A newer connection replaces the old one without emitting a stale `participant_left`.

After a successful REST end, every connected participant receives the notification before its socket is closed:

```json
{"version":1,"type":"call_ended","data":{"ended_by_user_id":1},"occurred_at":"2026-09-16T12:34:56Z"}
```

## Limits and deployment

Incoming WebSocket messages are limited to 16 KiB. Text frames carry the JSON protocol above; audio frames are binary and must have the exact 664-byte format above. Messages, mute states, and audio are ephemeral and are not persisted.

The hub is in memory and coordinates connections only inside one backend process. Deployments with multiple backend instances require sticky routing or a shared pub/sub and presence implementation; the current protocol layer alone does not synchronize instances.

## End-to-end verification

With PostgreSQL, backend, and frontend running, the live two-session browser test can be executed with:

```bash
LIVE_E2E=1 LIVE_API_ORIGIN=http://127.0.0.1:8080 \
  npx playwright test tests/realtime-live.spec.ts --project=desktop
```

The scenario creates two different users, opens an isolated browser context for each one, exchanges messages in both directions, propagates mute and language changes, and verifies that ending the call redirects both sessions.
