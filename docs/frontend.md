# Frontend

## Scope

The application uses backend authentication and persisted calls, captures PCM16 microphone audio through an AudioWorklet, exchanges call events and audio over an authenticated WebSocket, displays provider transcriptions, and plays translated PCM at 24 kHz. The public landing-page conversation preview uses local sample content; authenticated screens use backend data.

All six routes can be loaded directly. Login and registration call the backend, and the bearer token persists locally so sessions survive a tab close. Registration validates matching passwords before submitting. Call creation accepts an optional meeting name, defaulting to `Meeting`, and persists it with the room.

## Structure

```text
frontend/
  src/
    components/
      layout/        Public, application, and authentication layouts
      ui/            Buttons, waves, avatar, language selector, status, reveals
      call/          Participants, transcript, chat, invitation, and controls
    contexts/        Shared audio and language preferences
    data/            Supported languages and landing-page preview content
    hooks/           Authentication, audio, real-time events, and reduced motion
    pages/           Landing, Login, Register, Home, CallSetup, Call
    routes/          Central route configuration, titles, and focus handling
    types/           Shared application and protocol types
    App.tsx
    main.tsx
    styles.css
  tests/             Playwright browser tests
  playwright.config.ts
```

React Context holds the small amount of preference state shared between screens. No additional state-management or animation framework is needed.

## Application Behavior

- Creating a call sends its name and language preferences to the backend and opens `/call/:code/setup` using the canonical room code returned by the API.
- Joining accepts room codes made of letters, numbers, and hyphens between words, normalized to lowercase.
- Setup requests a real browser microphone, lists available devices, supports listen-only entry, and carries language preferences into the room.
- The call screen remains in an honest waiting state until real transcription events arrive. No sample transcript is injected into authenticated calls.
- Microphone capture, translated audio playback, volume, language changes, participants, chat, and call termination use the real-time backend flow.
- Ending opens a keyboard-accessible native confirmation dialog. Cancel or Escape returns to the conversation; confirmation returns to Home.

## Design and Accessibility

IBM Plex Mono is bundled locally through Fontsource. The interface uses editorial typography, thin dividers, and a restrained signal accent. Lucide supplies icons. The hero's full-width canvas generates a procedural audio signal without external imagery or device access.

CSS handles waveforms, transitions, and reveals; Intersection Observer activates sections as they enter the viewport. The canvas stops scheduling frames when offscreen or when the document is hidden. Reduced motion stops ambient animation while keeping all content and controls available.

Forms have labels, visible focus, browser validation, and inline errors. Route changes focus the main landmark. Icon controls have accessible names and native title tooltips. Mobile stacks the call, transcript, participants, and chat; fixed call controls stay reachable with bottom content spacing.

## Verification

Run `npm run build`, `npm run typecheck`, and `npm run test:e2e` from `frontend/`. Install the browser once with `npx playwright install chromium`.

The suite checks 1440x900, 768x1024, and 390x844 viewports, plus a 320px layout check. It exercises all routes with intercepted backend responses, checks console errors and external asset requests, blocks microphone access, verifies canvas pixels and animation, tests reduced motion, and captures screenshots for visual review.

The checks use Chromium. Physical devices and other browser engines still require manual validation. Serving the production build requires an SPA fallback to `index.html` for deep links.
