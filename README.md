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
- Go 1.25 or newer
- Docker and Docker Compose

Go is only required when running the backend directly on the host. Docker can be used instead.

## Environment

Copy `.env.example` to `.env` and adjust values when needed.

```bash
cp .env.example .env
```

Set `JWT_SECRET` to a unique value of at least 32 bytes before starting the
backend. Generate one locally, keep it only in `.env`, and rotate it if it is
ever exposed:

```powershell
$bytes = [byte[]]::new(48)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[Convert]::ToBase64String($bytes)
```

The Compose configuration exposes only the frontend to the LAN. PostgreSQL and
the backend API bind to `127.0.0.1` on the host and remain available to the
frontend through Docker's internal network.

### Manual Gemini setup

The following steps cannot be automated by the repository:

1. Create or select a Google Cloud project, enable Vertex AI and grant the backend
   identity access to the preview model `gemini-3.5-live-translate-preview`.
   Model availability, billing and regional restrictions are controlled by Google.
2. Use Application Default Credentials (ADC), via an attached service account,
   `gcloud auth application-default login`, or a local credentials file. Configure:

   ```dotenv
   TRANSLATION_PROVIDER=gemini
   GOOGLE_CLOUD_PROJECT=your-project
   TRANSLATION_MODEL=gemini-3.5-live-translate-preview
   ```

3. For local Docker, set `GOOGLE_APPLICATION_CREDENTIALS` to the host credentials
   path and use `docker compose -f docker-compose.yml -f docker-compose.adc.yml up --build`.
   Alternatively, export the variables
   in the shell that starts `go run ./cmd/server`. A successful backend startup
   prints `translation provider: gemini`.
4. Optionally validate ADC, model access and Live WebSocket handshake directly
   from `backend/`. This test contacts Google and is skipped during normal tests:

   ```bash
   GEMINI_LIVE_INTEGRATION=1 \
   GOOGLE_CLOUD_PROJECT="your-project" \
   go test ./internal/translation -run TestGeminiLiveIntegration -v
   ```

5. Open the same call as two different users, choose different spoken/heard
   languages, allow microphone access and speak. For PT↔EN, configure A to speak/hear
   Portuguese and B to speak/hear English. The listener receives 16 kHz binary
   translated PCM (resampled from Gemini's 24 kHz); the call screen replaces its demo text when real input
   and output transcription events arrive.

Credentials stay in the backend. Vertex ADC takes precedence when a project is
configured; API-key compatibility remains available without a project. Provider
errors produce `translation_unavailable` without exposing credentials or relaying
untranslated speech. See [translation documentation](docs/translation.md) for
official references, lifecycle behavior, tests, and preview limitations.

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

When Docker prints a `Network` URL such as `http://172.19.0.4:5173`, **do not
use it on another device**: it is the frontend container's private Docker IP.
Use the LAN IP of the computer that runs Docker instead. On Windows, find it
with:

```powershell
Get-NetIPAddress -AddressFamily IPv4 |
  Where-Object { $_.AddressState -eq 'Preferred' } |
  Format-Table InterfaceAlias, IPAddress
```

For example, if the active Wi-Fi or Ethernet adapter shows `192.168.18.7`, open
`http://192.168.18.7:5173/` from the phone, tablet, or another computer. Both
devices must be on the same LAN; a guest Wi-Fi network with client isolation
will prevent this connection.

Vite listens on all interfaces and proxies `/api`, including WebSockets, to the
backend. Keep `VITE_API_BASE_URL` unset or empty to use this same-origin path.
The proxy defaults to `http://127.0.0.1:8080`; Compose sets `API_PROXY_TARGET` to
`http://backend:8080`. For a different backend port when running locally, set
`API_PROXY_TARGET` in the environment of the Vite process.

When using Docker, the frontend service explicitly binds Vite to `0.0.0.0` and
publishes port `5173` on the host. If another device still cannot connect, allow
the configured `FRONTEND_PORT` through the host firewall. On Windows, run this
in an Administrator PowerShell to allow only devices on the private local
subnet:

```powershell
New-NetFirewallRule `
  -DisplayName "Symphonia Frontend (LAN)" `
  -Direction Inbound `
  -Action Allow `
  -Protocol TCP `
  -LocalPort 5173 `
  -Profile Private `
  -RemoteAddress LocalSubnet
```

This setup exposes the frontend only on the local network. Publishing it to the
internet requires a reverse proxy with HTTPS and authentication; do not expose
the development Vite server directly.

Login and calls require the backend and PostgreSQL to be running as well.
If the page cannot load, verify that the server is running, port 5173 is allowed
by the host firewall, and the Wi-Fi network permits communication between devices.
Microphone access from another device requires HTTPS with a certificate trusted
by that device; browsers generally block microphone capture on plain HTTP LAN IPs.
Use plain HTTP only on a trusted local network: bearer tokens and audio traffic
are not encrypted. For any untrusted network or internet deployment, terminate
HTTPS at a reverse proxy and serve the frontend through that proxy.

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
