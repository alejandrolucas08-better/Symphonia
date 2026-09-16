# Symphonia

Symphonia is a web application for real-time communication between two people who speak different languages.

The MVP goal is an audio call for up to two participants, with live speech transcription and translation.

This repository currently contains only the initial project foundation. Product features such as authentication, calls, WebSocket signaling, audio capture, and Gemini Live Translate are intentionally not implemented yet.

## Stack

- Frontend: React, TypeScript, Vite
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
