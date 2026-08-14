GO ?= go
GRADLE ?= ./examples/library-only/gradlew
VERSION ?= dev
RELEASE_VERSION ?= v0.1.0-beta.1
RELEASE_DIR ?= dist/$(RELEASE_VERSION)

.PHONY: build test test-race test-upgrade-assistant-fixture test-migration-smoke test-upgrade-agent-smoke vet vuln demo check release release-check example-build example-verify

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "-X=main.version=$(VERSION)" -o bin/aargrade ./cmd/aargrade

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-upgrade-assistant-fixture:
	$(GO) test ./tests/integration -run '^TestUpgradeAssistantReproHostLifecycle$$' -v

test-migration-smoke:
	./scripts/migration-smoke.sh

test-upgrade-agent-smoke:
	./scripts/upgrade-agent-smoke.sh

vet:
	$(GO) vet ./...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

demo:
	./scripts/demo.sh

check: test vet demo

release:
	./scripts/build-release.sh "$(RELEASE_VERSION)" "$(RELEASE_DIR)"

release-check:
	@release_tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$${release_tmp}"' EXIT; \
	./scripts/build-release.sh v0.0.0-test.1 "$${release_tmp}"

example-build:
	$(GRADLE) -p examples/library-only :sdk:assembleRelease --no-daemon

example-verify: example-build
	$(GO) run ./cmd/aargrade verify \
		--baseline-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar \
		--candidate-aar examples/library-only/sdk/build/outputs/aar/sdk-release.aar
