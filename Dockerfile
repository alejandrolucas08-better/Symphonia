FROM node:22-alpine AS frontend-build
WORKDIR /build/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend-build
WORKDIR /build/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/symphonia ./cmd/server

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates \
    && addgroup -S symphonia && adduser -S -G symphonia symphonia
WORKDIR /app
COPY --from=backend-build /out/symphonia /app/symphonia
COPY --from=frontend-build /build/frontend/dist /app/public
ENV APP_ENV=production FRONTEND_DIST=/app/public
USER symphonia
EXPOSE 8080
ENTRYPOINT ["/app/symphonia"]
