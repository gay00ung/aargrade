GO ?= go
GRADLE ?= ./examples/library-only/gradlew

.PHONY: build test test-race vet vuln demo check example-build example-verify

build:
	mkdir -p bin
	$(GO) build -trimpath -o bin/aargrade ./cmd/aargrade

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

demo:
	./scripts/demo.sh

check: test vet demo

example-build:
	$(GRADLE) -p examples/library-only :sdk:assembleRelease --no-daemon

example-verify: example-build
	$(GO) run ./cmd/aargrade verify \
		--baseline-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar \
		--candidate-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar
