# Symphonia

Symphonia is a web application for real-time communication between two people who speak different languages.

The MVP goal is an audio call for up to two participants, with live speech transcription and translation.

The project includes backend authentication and calls plus an authenticated WebSocket for transient call events and best-effort PCM audio relay between two participants. Browser capture and playback support is in active development; Gemini Live Translate is not integrated.

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

## Running the Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend runs at `http://localhost:5173` by default.

The prototype works independently of the backend, Docker, and environment variables. Use fictitious form values. Reloading the page resets the demo profile, settings, and chat.

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
