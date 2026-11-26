# minectl — Minecraft server management via Docker
BINARY := minectl
MAIN   := ./cmd/minectl
GO     := go

.PHONY: build run test e2e lint tidy install release-dry

build:
	$(GO) build -o bin/$(BINARY) $(MAIN)

run:
	$(GO) run $(MAIN)

test:
	$(GO) test -race ./...

e2e:
	$(GO) test -tags e2e -v -count=1 -timeout 20m ./test/e2e/

lint:
	golangci-lint run

tidy:
	$(GO) mod tidy

install: build
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)

release-dry:
	goreleaser release --snapshot --clean
