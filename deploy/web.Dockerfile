# Build the frontend against the REAL API (mocks disabled) and serve it via Caddy,
# which also reverse-proxies /api, /sub, /swagger and health to the backend.
# Build context is the repo root (needs frontend/ and deploy/).

FROM node:22-alpine AS build
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Phase 7: flip the mock service worker off so the app hits the real backend.
ENV VITE_USE_MOCKS=false
RUN npm run build

FROM caddy:2-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv
