.PHONY: help web build run dev test lint size e2e image

GO_PKGS := ./cmd/... ./internal/...
VERSION ?= dev

help:
	@echo "make web     build the frontend into internal/webui/dist"
	@echo "make build   build the frontend, then bin/huginn"
	@echo "make run     run bin/huginn with ./data as the data directory"
	@echo "make dev     run the Go server (:8080) and the Vite dev server (:5173)"
	@echo "make test    run the Go tests"
	@echo "make lint    gofmt, go vet and svelte-check"
	@echo "make size    check the frontend bundle against its size budget"
	@echo "make e2e     browser and accessibility checks (run make build first)"
	@echo "make image   build the Docker image as huginn:dev"

web:
	cd frontend && npm ci && npm run build

build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/huginn ./cmd/huginn

run: build
	HUGINN_DATA_DIR=./data HUGINN_ADDR=:8080 ./bin/huginn

dev:
	cd frontend && npm install
	@trap 'kill 0' INT TERM EXIT; \
	HUGINN_DATA_DIR=./data go run ./cmd/huginn & \
	cd frontend && npm run dev

test:
	go test -race $(GO_PKGS)

lint:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; echo "gofmt: files above need formatting"; exit 1; }
	go vet $(GO_PKGS)
	cd frontend && npm run check

size:
	cd frontend && npm run size

e2e:
	e2e/run.sh

image:
	docker build --build-arg VERSION=$(VERSION) -t huginn:dev .
