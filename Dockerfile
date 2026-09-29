# syntax=docker/dockerfile:1

# Stage 1: Build SvelteKit Frontend
FROM oven/bun:1 AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile

COPY frontend ./
RUN bun run build

# Stage 2: Build Golang Single Binary
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app/backend

RUN apk add --no-cache git

COPY backend/ ./
# Ensure freshly built frontend dist is placed inside dist/
COPY --from=frontend-builder /app/backend/dist ./dist

# Build single static binary with embedded SPA frontend
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/gopod ./cmd/server

# Stage 3: Minimal Production Container
FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=backend-builder /app/gopod /app/gopod

ENV PORT=8085
EXPOSE 8085

ENTRYPOINT ["/app/gopod"]
