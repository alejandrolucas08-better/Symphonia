# Symphonia

Symphonia is a web application for real-time communication between two people who speak different languages.

The MVP goal is an audio call for up to two participants, with live speech transcription and translation.

The project includes backend authentication and calls, browser microphone capture and playback, and an authenticated WebSocket that streams PCM audio through Gemini Live Translate when the participants use different languages.

## Stack

- Frontend: React, TypeScript, Vite, React Router, Lucide React
- Backend: Go
- Database: PostgreSQL
- Real-time communication: WebSocket
- Audio: Web Audio API / MediaStream
- Translation: Gemini Live Translate
- Development environment: Docker Compose

## Project Structure

```text
symphonia/
├── frontend/
├── backend/
├── docs/
├── .env.example
├── .gitignore
├── README.md
└── docker-compose.yml
```

## Requirements

- Node.js 22 or newer
- npm 12 or newer
- Go 1.23 or newer
- Docker and Docker Compose

Go is only required when running the backend directly on the host. Docker can be used instead.

## Environment

Copy `.env.example` to `.env` and adjust values when needed.

```bash
cp .env.example .env
```

### Manual Gemini setup

The following steps cannot be automated by the repository:

1. Create or select a Google AI project and obtain an API key that has access to
   the Gemini API and the preview model `gemini-3.5-live-translate-preview`.
   Model availability, billing and regional restrictions are controlled by Google.
2. Keep the key only in the local `.env` file. Never place it in source code or
   commit it. Configure:

   ```dotenv
   TRANSLATION_PROVIDER=gemini
   GEMINI_API_KEY=your-real-key
   TRANSLATION_MODEL=gemini-3.5-live-translate-preview
   ```

3. Start all services with `docker compose up --build`, or export the variables
   in the shell that starts `go run ./cmd/server`. A successful backend startup
   prints `translation provider: gemini`.
4. Optionally validate the key, model access and Live WebSocket handshake directly
   from `backend/`. This test contacts Google and is skipped during normal tests:

   ```bash
   GEMINI_LIVE_INTEGRATION=1 \
   GEMINI_API_KEY="your-real-key" \
   go test ./internal/translation -run TestGeminiLiveIntegration -v
   ```

5. Open the same call as two different users, choose different spoken/heard
   languages, allow microphone access and speak. The listener receives 24 kHz
   translated PCM audio; the call screen replaces its demo text when real input
   and output transcription events arrive.

The backend sends the API key directly to Google over TLS; it is never exposed to
the browser. If Google rejects setup, inspect the backend log for `gemini session
setup failed`. Typical manual causes are an invalid/restricted key, unavailable
preview model, disabled billing/API, or an unsupported region.

## Running the Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend runs at `http://localhost:5173` by default.

### Access from another device on the local network

Start the frontend with `npm run dev` from `frontend/`, or start the complete
stack with `docker compose up --build`. On a device connected to the same local
network, open `http://<computer-LAN-IP>:5173` (use the published `FRONTEND_PORT`
instead if changed in Compose). `localhost` on a phone refers to the phone,
not the computer running Symphonia.

Vite listens on all interfaces and proxies `/api`, including WebSockets, to the
backend. Keep `VITE_API_BASE_URL` unset or empty to use this same-origin path.
The proxy defaults to `http://127.0.0.1:8080`; Compose sets `API_PROXY_TARGET` to
`http://backend:8080`. For a different backend port when running locally, set
`API_PROXY_TARGET` in the environment of the Vite process.

Login and calls require the backend and PostgreSQL to be running as well.
If the page cannot load, verify that the server is running, port 5173 is allowed
by the host firewall, and the Wi-Fi network permits communication between devices.
Microphone access from another device requires HTTPS with a certificate trusted
by that device; browsers generally block microphone capture on plain HTTP LAN IPs.

The public landing-page demonstration uses local sample phrases. Authentication,
calls, microphone streaming and Gemini translation require the backend and database.

Routes: `/`, `/login`, `/register`, `/home`, `/call/:id/setup`, and `/call/:id`.

### Frontend Verification

Run from `frontend/`:

```bash
npm run build
npm run typecheck
npx playwright install chromium
npm run test:e2e
```

The browser tests cover desktop, tablet, mobile, route navigation, mock forms, call controls, chat, animated signal rendering, and reduced motion. Playwright starts a local Vite server when needed. Test screenshots and traces are written to the ignored `frontend/test-results/` directory.

See [frontend implementation notes](docs/frontend.md) for the component structure and demo behavior.

## Running the Backend

```bash
cd backend
go run ./cmd/server
```

The backend runs at `http://localhost:8080` by default.

Health check:

```bash
curl http://localhost:8080/healthz
```

## Running with Docker Compose

```bash
docker compose up --build
```

Services:

- Frontend: `http://localhost:5173`
- Backend: `http://localhost:8080`
- PostgreSQL: `localhost:5432`
