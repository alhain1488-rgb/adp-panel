# Developer convenience targets. The canonical API contract is docs/openapi.yaml;
# the backend embeds a committed copy and the frontend generates types from it.

.PHONY: sync-openapi gen-types test smoke up down install logs restart

# One-command install / update on a Debian/Ubuntu server (see install.sh).
install:
	sudo bash install.sh

# Copy the canonical spec into the backend (for go:embed) and regenerate frontend types.
sync-openapi:
	cp docs/openapi.yaml backend/internal/httpapi/openapi.yaml
	cd frontend && npm run gen:api

gen-types:
	cd frontend && npm run gen:api

# Run all backend + frontend tests.
test:
	cd backend && go vet ./... && gofmt -l . && go test ./...
	cd frontend && npm run build && npm run lint && npm run test

# Full local stack.
up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

restart:
	docker compose restart

# End-to-end smoke against a running stack.
smoke:
	./scripts/smoke.sh
