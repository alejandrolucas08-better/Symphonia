# Frontend Visual Prototype

## Scope

This stage implements the visual experience only. There are no backend requests, real authentication, real calls, microphone access, camera/video elements, speech recognition, translation services, or persistence. Vite's development WebSocket is only used for hot module replacement.

All six routes can be loaded directly. Login uses fictitious values and opens the default Alejandro profile. Registration validates matching passwords and uses the supplied display name for the current in-memory session. Passwords are never saved or sent. Refreshing resets the session.

## Structure

```text
frontend/
  src/
    components/
      layout/        Public, application, and authentication layouts
      ui/            Buttons, waves, avatar, language selector, status, reveals
      call/          Participants, transcript, local chat, and controls
    contexts/        In-memory demo profile and preferences
    data/            Participants, phrases, languages, and initial messages
    hooks/           Reduced-motion preference and conversation timing
    pages/           Landing, Login, Register, Home, CallSetup, Call
    routes/          Central route configuration, titles, and focus handling
    types/           Shared demo types
    App.tsx
    main.tsx
    styles.css
  tests/             Playwright browser tests
  playwright.config.ts
```

The existing stylesheet location and unused extension directories were preserved. React Context holds only the small amount of state shared between screens. No state-management or animation framework is needed.

## Demo Behavior

- Create opens `/call/demo-room/setup`. Join accepts a room code made of letters, numbers, and hyphens between words, normalized to lowercase.
- Setup offers a simulated microphone and spoken/listening language choices. Preferences carry into the room.
- Alejandro (or the supplied profile name) and Mateus alternate every seven seconds. The translation appears 1.2 seconds after the original. The next-phrase control also works with animation paused.
- Mateus speaks English; the current user's spoken language and the displayed translation target follow the chosen preferences. Phrases are predefined in Portuguese, English, Spanish, and French. No actual translation happens.
- Microphone and speaker controls change local state. The volume slider does not produce or change actual audio.
- Chat messages stay within the mounted room. Sending does not contact another user. Leaving the room clears its messages and timer.
- Ending opens a keyboard-accessible native confirmation dialog. Cancel or Escape returns to the conversation; confirmation returns to Home.

## Design and Accessibility

IBM Plex Mono is bundled locally through Fontsource. The interface uses the requested palette, editorial typography, thin dividers, and a small warm signal accent. Lucide supplies icons. The hero's full-width canvas generates a procedural audio signal without external imagery or device access.

CSS handles waveforms, transitions, and reveals; Intersection Observer activates sections as they enter the viewport. The canvas stops scheduling frames when offscreen or when the document is hidden. Reduced motion stops ambient animation and automatic phrase rotation while keeping all content and manual controls available.

Forms have labels, visible focus, browser validation, and inline errors. Route changes focus the main landmark. Icon controls have accessible names and native title tooltips. Mobile stacks the call, transcript, participants, and chat; the fixed call controls stay reachable with bottom content spacing.

## Verification

Run `npm run build`, `npm run typecheck`, and `npm run test:e2e` from `frontend/`. Install the browser once with `npx playwright install chromium`.

The suite checks 1440x900, 768x1024, and 390x844 viewports, plus a 320px layout check. It exercises all routes and mock workflows, checks console errors/warnings and external asset requests, blocks microphone access, verifies canvas pixels and animation, tests reduced motion and speaker timing, and captures screenshots for visual review.

The checks use Chromium. Physical devices and other browser engines have not been validated in this stage. Serving the production build requires an SPA fallback to `index.html` for deep links.
