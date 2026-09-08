.PHONY: dev dev-api dev-frontend build build-go clean install

# The root .env reaches the Go side through godotenv (cmd/root.go:29), but Make
# does not read .env files and Next only reads them from frontend/ — so without
# this the frontend never saw API_URL or any NEXT_PUBLIC_*. next.config.ts fell
# through to its localhost:8080 default (right by luck) and ShareLinkModal.tsx
# threw "NEXT_PUBLIC_BASE_URL is not defined". NEXT_PUBLIC_* are inlined at
# compile time, so build-frontend needs them as much as dev-frontend does.
#
# Allowlisted, not re-exported wholesale: SSHCM_SECRET_KEY and SSHCM_DB_DSN
# live in the same file and have no business in the Next process.
# ponytail: a value containing spaces would need real quoting; every var here
# is a URL or a flag. Swap in `set -a; . ./.env; set +a` if that stops holding.
FRONTEND_ENV := $(shell sed -nE 's/^(API_URL|NEXT_PUBLIC_[A-Z0-9_]+)=(.*)$$/\1=\2/p' .env 2>/dev/null)

# Development: run Go API on :8080 and Next.js dev server on :3000 in parallel
# Next.js proxies /api/* to Go via rewrites in next.config.ts
dev:
	$(MAKE) dev-api & $(MAKE) dev-frontend & wait

dev-api:
	go run . web --port 8080

dev-frontend:
	cd frontend && env $(FRONTEND_ENV) npm run dev -- -p 3000 --hostname 0.0.0.0

# Production: build Go binary (frontend served separately via `npm start` in frontend/)
build: build-frontend build-go

build-frontend:
	cd frontend && npm ci && env $(FRONTEND_ENV) npm run build

build-go:
	go build -o sshcm .

# Run production (Go API + Next.js server)
start:
	$(MAKE) start-api & $(MAKE) start-frontend & wait

start-api:
	./sshcm web --port 8080

start-frontend:
	cd frontend && env $(FRONTEND_ENV) npm start -- -p 3000 --hostname 0.0.0.0

clean:
	rm -rf frontend/.next sshcm

install: build-go
	cp sshcm $(GOPATH)/bin/sshcm 2>/dev/null || cp sshcm ~/go/bin/sshcm
