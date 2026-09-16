# Symphonia frontend

## Browser audio

Calls capture the selected microphone with `getUserMedia` and an `AudioWorklet`. The public worklet downmixes and resamples the browser's actual audio-context rate to mono 16 kHz PCM, emitting 320-sample packets. The main thread adds the fixed 24-byte `SA` header and sends 664-byte binary WebSocket frames. Incoming valid PCM frames are played through Web Audio with a short bounded jitter schedule and the existing volume control.

The setup screen only requests microphone permission after an explicit action or when entering a call. Every temporary setup stream is stopped. If permission or device validation fails, the user can retry or explicitly enter in listen-only mode. AudioWorklet is required for capture; there is no MediaRecorder or lossy fallback.

Append `?audioDiagnostics=1` to a call URL to expose `window.__symphoniaAudioDiagnostics` with outgoing/incoming frame counts and the live-track count. It never contains sample data.

## Verification

```bash
npm run typecheck
npm run build
npm run test:e2e
```

The optional real-backend test launches Chromium with fake microphone input:

```bash
LIVE_E2E=1 LIVE_API_ORIGIN=http://127.0.0.1:8080 \
  npx playwright test tests/realtime-live.spec.ts --project=desktop
```
