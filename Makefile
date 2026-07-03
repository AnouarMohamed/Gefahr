.PHONY: build test test-race coverage test-integration test-integration-race load-check deploy-check docs docs-preview check acceptance

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
COVERAGE_MIN ?= 85.0
LDFLAGS = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/goproxy ./cmd/goproxy

test:
	go test ./...

test-race:
	go test -race ./...

coverage:
	go test ./... -coverprofile=coverage.txt
	go tool cover -func=coverage.txt | awk -v min="$(COVERAGE_MIN)" '/^total:/ { gsub("%", "", $$3); if ($$3 + 0 < min) { printf "coverage %.1f%% below %.1f%%\n", $$3, min; exit 1 } printf "coverage %.1f%% >= %.1f%%\n", $$3, min }'

test-integration:
	go test -tags=integration ./test/integration

test-integration-race:
	go test -race -tags=integration ./test/integration

load-check:
	GOPROXY_ADMIN_TOKEN=$${GOPROXY_ADMIN_TOKEN:-compose-demo-token} go run ./cmd/loadcheck

deploy-check:
	./scripts/deploy-check.sh

docs:
	cd gefahr-docs && npm ci && npm run build

docs-preview:
	cd gefahr-docs && npm run preview

check:
	test -z "$$(gofmt -l $$(git ls-files '*.go'))"
	go vet ./...
	go test -race ./...

acceptance: check deploy-check test-integration-race
