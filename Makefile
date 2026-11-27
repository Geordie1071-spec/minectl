# minectl — Minecraft server management via Docker
BINARY := minectl
MAIN   := ./cmd/minectl
GO     := go

.PHONY: build web panel run test e2e smoke lint tidy install release-dry

build:
	$(GO) build -o bin/$(BINARY) $(MAIN)

# Build the web panel; `make panel` then embeds it in the binary.
web:
	npm --prefix web ci
	npm --prefix web run build

panel: web build

smoke: panel
	npm --prefix web run smoke

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
