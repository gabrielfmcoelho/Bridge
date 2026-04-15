.PHONY: dev dev-api dev-frontend build build-go clean install

# Development: run Go API on :8080 and Next.js dev server on :3000 in parallel
# Next.js proxies /api/* to Go via rewrites in next.config.ts
dev:
	$(MAKE) dev-api & $(MAKE) dev-frontend & wait

dev-api:
	go run . web --port 8080

dev-frontend:
	cd frontend && npm run dev -- -p 3000

# Production: build Go binary (frontend served separately via `npm start` in frontend/)
build: build-frontend build-go

build-frontend:
	cd frontend && npm ci && npm run build

build-go:
	go build -o sshcm .

# Run production (Go API + Next.js server)
start:
	$(MAKE) start-api & $(MAKE) start-frontend & wait

start-api:
	./sshcm web --port 8080

start-frontend:
	cd frontend && npm start -- -p 3000

clean:
	rm -rf frontend/.next sshcm

install: build-go
	cp sshcm $(GOPATH)/bin/sshcm 2>/dev/null || cp sshcm ~/go/bin/sshcm
