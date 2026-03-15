# minectl — Minecraft server management via Docker
BINARY := minectl
MAIN   := ./cmd/minectl
GO     := go

.PHONY: build run test lint tidy install release-dry

build:
	$(GO) build -o bin/$(BINARY) $(MAIN)

run:
	$(GO) run $(MAIN)

test:
	$(GO) test ./...

lint:
	golangci-lint run

tidy:
	$(GO) mod tidy

install: build
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)

release-dry:
	goreleaser release --snapshot --clean
