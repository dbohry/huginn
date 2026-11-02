.PHONY: help build run test lint image

GO_PKGS := ./cmd/... ./internal/...
VERSION ?= dev

help:
	@echo "make build   build bin/huginn"
	@echo "make run     run bin/huginn with ./data as the data directory"
	@echo "make test    run the Go tests"
	@echo "make lint    gofmt and go vet"
	@echo "make image   build the Docker image as huginn:dev"

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/huginn ./cmd/huginn

run: build
	HUGINN_DATA_DIR=./data HUGINN_ADDR=:8080 ./bin/huginn

test:
	go test -race $(GO_PKGS)

lint:
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; echo "gofmt: files above need formatting"; exit 1; }
	go vet $(GO_PKGS)

image:
	docker build --build-arg VERSION=$(VERSION) -t huginn:dev .
