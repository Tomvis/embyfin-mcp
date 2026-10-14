# recipes use bash for pipefail support (ubuntu's default sh is dash)
SHELL := /bin/bash

GIT_COMMIT=$(shell git describe --always --long --dirty)
# falls back to dev when there is no tag yet (the || must wrap git, not sed, or it never fires)
GIT_VERSION=$(shell (git describe --tags --dirty 2>/dev/null || echo dev) | sed 's/-\([0-9]*\)-g/+\1@g/')
TEST_TIMEOUT?=30m
empty:=
space:=$(empty) $(empty)

# dev tool binaries are built into .tools/bin (gitignored) from the versions pinned in
# .tools/go.mod - the single source of truth for make and CI; dependabot keeps them updated.
# overridable because binaries cannot execute from a noexec mount (e.g. an smb checkout):
# make TOOLS_BIN=~/.cache/embyfin-mcp/bin lint
TOOLS_BIN?=.tools/bin
ACTIONLINT=$(TOOLS_BIN)/actionlint
GOFUMPT=$(TOOLS_BIN)/gofumpt
GOLANGCI_LINT=$(TOOLS_BIN)/golangci-lint

# non-Go tools also live in .tools/bin at pinned versions, but the pins are here (dependabot
# cannot bump them): shellcheck, typos and zizmor are static binaries downloaded from their github
# releases, yamllint is pip installed into a repo-local venv and markdownlint-cli2 is npm installed
# into a repo-local prefix. all rebuild when this makefile changes.
MARKDOWNLINT_CLI2_VERSION=0.23.2
SHELLCHECK_VERSION=v0.11.0
TYPOS_VERSION=v1.50.1
YAMLLINT_VERSION=1.38.0
ZIZMOR_VERSION=v1.30.1
MARKDOWNLINT=$(TOOLS_BIN)/markdownlint-cli2
SHELLCHECK=$(TOOLS_BIN)/shellcheck
TYPOS=$(TOOLS_BIN)/typos
YAMLLINT=$(TOOLS_BIN)/yamllint
ZIZMOR=$(TOOLS_BIN)/zizmor

# golangci-lint with the azproviderlint module plugin compiled in (.tools/.custom-gcl.yml);
# lint runs use this binary, the plain go.mod one exists to bootstrap `golangci-lint custom`
GOLANGCI_LINT_MODULES=$(TOOLS_BIN)/golangci-with-modules

# one rule builds any Go tool: the import path comes from the tool directives in .tools/go.mod
# (via go list tool), so the makefile never repeats it - add a tool there and a variable above
$(TOOLS_BIN)/%: .tools/go.mod .tools/go.sum
	@echo "==> building $* (version pinned in .tools/go.mod)..."
	@mkdir -p $(TOOLS_BIN)
	@cd .tools && go build -o $(abspath $@) $$(go list tool | grep "/$*$$")

# explicit rules take precedence over the pattern rule above for the non-Go tools.
# `golangci-lint custom` always writes to .tools/bin (destination in .custom-gcl.yml), so move
# the result when TOOLS_BIN points elsewhere (e.g. a local dir because the checkout is noexec)
$(GOLANGCI_LINT_MODULES): .tools/.custom-gcl.yml $(GOLANGCI_LINT)
	@echo "==> building golangci-lint with plugins (versions pinned in .tools/.custom-gcl.yml)..."
	@cd .tools && mkdir -p bin && $(abspath $(GOLANGCI_LINT)) custom
	@if [ "$(abspath $(TOOLS_BIN))" != "$(abspath .tools/bin)" ]; then mv .tools/bin/golangci-with-modules $@; fi

$(SHELLCHECK): makefile
	@echo "==> downloading shellcheck $(SHELLCHECK_VERSION)..."
	@mkdir -p $(TOOLS_BIN)
	@os=$$(uname | tr 'A-Z' 'a-z'); arch=$$(uname -m); [ "$$arch" = "arm64" ] && arch=aarch64; \
		curl -sSfL "https://github.com/koalaman/shellcheck/releases/download/$(SHELLCHECK_VERSION)/shellcheck-$(SHELLCHECK_VERSION).$$os.$$arch.tar.xz" \
		| tar -xJ -O shellcheck-$(SHELLCHECK_VERSION)/shellcheck > $@ && chmod +x $@

$(YAMLLINT): makefile
	@command -v python3 >/dev/null || (echo "python3 is required to install yamllint (macOS: xcode CLT; Debian/Ubuntu: apt install python3-venv)" && exit 1)
	@echo "==> installing yamllint $(YAMLLINT_VERSION) into $(TOOLS_BIN)/../venv..."
	@mkdir -p $(TOOLS_BIN)
	@python3 -m venv $(TOOLS_BIN)/../venv && $(TOOLS_BIN)/../venv/bin/pip install -q yamllint==$(YAMLLINT_VERSION) && ln -sf ../venv/bin/yamllint $@

$(ZIZMOR): makefile
	@echo "==> downloading zizmor $(ZIZMOR_VERSION)..."
	@mkdir -p $(TOOLS_BIN)
	@case "$$(uname)" in Darwin) target=apple-darwin;; *) target=unknown-linux-gnu;; esac; \
		arch=$$(uname -m); [ "$$arch" = "arm64" ] && arch=aarch64; \
		curl -sSfL "https://github.com/zizmorcore/zizmor/releases/download/$(ZIZMOR_VERSION)/zizmor-$$arch-$$target.tar.gz" \
		| tar -xz -O zizmor > $@ && chmod +x $@

$(TYPOS): makefile
	@echo "==> downloading typos $(TYPOS_VERSION)..."
	@mkdir -p $(TOOLS_BIN)
	@case "$$(uname)" in Darwin) target=apple-darwin;; *) target=unknown-linux-musl;; esac; \
		arch=$$(uname -m); [ "$$arch" = "arm64" ] && arch=aarch64; \
		curl -sSfL "https://github.com/crate-ci/typos/releases/download/$(TYPOS_VERSION)/typos-$(TYPOS_VERSION)-$$arch-$$target.tar.gz" \
		| tar -xz -O ./typos > $@ && chmod +x $@

$(MARKDOWNLINT): makefile
	@command -v npm >/dev/null || (echo "npm is required to install markdownlint-cli2 (macOS: brew install node; Debian/Ubuntu: apt install npm)" && exit 1)
	@echo "==> installing markdownlint-cli2 $(MARKDOWNLINT_CLI2_VERSION) into $(TOOLS_BIN)/../npm..."
	@mkdir -p $(TOOLS_BIN)
	@npm install --silent --no-audit --no-fund --prefix $(TOOLS_BIN)/../npm markdownlint-cli2@$(MARKDOWNLINT_CLI2_VERSION) && ln -sf ../npm/node_modules/.bin/markdownlint-cli2 $@

default: fmt build

all: fmt build

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-24s\033[0m%s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Build
build: ## Compile embyfin-mcp with version info from git
	@echo "==> building..."
	go build -ldflags "-X github.com/katbyte/go-kt/version.GitCommit=${GIT_COMMIT} -X github.com/katbyte/go-kt/version.Version=${GIT_VERSION}"

install: ## Install embyfin-mcp into GOPATH/bin with version info from git
	@echo "==> installing..."
	go install -ldflags "-X github.com/katbyte/go-kt/version.GitCommit=${GIT_COMMIT} -X github.com/katbyte/go-kt/version.Version=${GIT_VERSION}" .

docker: ## Build the embyfin-mcp container image with version info from git
	@echo "==> building docker image..."
	docker build --build-arg VERSION=${GIT_VERSION} --build-arg COMMIT=${GIT_COMMIT} -t embyfin-mcp .

tools: $(ACTIONLINT) $(GOFUMPT) $(GOLANGCI_LINT) $(GOLANGCI_LINT_MODULES) $(MARKDOWNLINT) $(SHELLCHECK) $(TYPOS) $(YAMLLINT) $(ZIZMOR) ## Install all pinned dev tools into .tools/bin

##@ SDK generation (sdk/main.go)
generate: pandorest-import pandorest-generate ## Import each service's highest-versioned spec into api-defs/<service>-<version>/, then generate sdk/emby, sdk/jf and sdk/tmdb

pandorest-import: ## Import api-defs/<service>-openapi-<version>.json (the highest version of each) into api-defs/<service>-<version>/, applying the workarounds
	@echo "==> importing the OpenAPI specs into api-defs/<service>-<version>/..."
	go run ./sdk import

pandorest-generate: ## Generate sdk/emby, sdk/jf and sdk/tmdb from api-defs/<service>-<version>/
	@echo "==> generating sdk/emby, sdk/jf and sdk/tmdb from api-defs/<service>-<version>/..."
	go run ./sdk generate

spec-refresh: ## Pull the latest server images, vendor each server's current OpenAPI document and TMDB's as of today under its version, regenerate, and print the API diff
	@.scripts/spec-refresh.sh

pandorest-diff: ## Report what the vendored specs change against their checked-in definitions
	@go run ./sdk diff -quiet

##@ Formatting
fmt: $(GOFUMPT) $(GOLANGCI_LINT) ## Fix Go formatting (gofmt, gofumpt, goimports)
	@echo "==> Fixing source code with gofmt..."
	find . -name '*.go' | grep -v vendor | xargs gofmt -s -w
	@echo "==> Fixing source code with gofumpt..."
	find . -name '*.go' | grep -v vendor | xargs $(GOFUMPT) -w
	@echo "==> Fixing imports with golangci-lint (goimports)..."
	$(GOLANGCI_LINT) fmt -E goimports ./...

goimports: $(GOLANGCI_LINT) ## Fix imports with golangci-lint (goimports)
	@echo "==> Fixing imports with golangci-lint (goimports)..."
	$(GOLANGCI_LINT) fmt -E goimports ./...

##@ Linting & Dependencies
# markdown checked by markdownlint: every .md the repo owns
MARKDOWN_INPUTS='*.md' 'docs/*.md' 'api-defs/**/*.md' 'sdk/*.md' '.github/**/*.md'

lint: $(GOLANGCI_LINT_MODULES) ## Check source code with the golangci linters (incl. azproviderlint)
	@echo "==> Checking source code against linters..."
	$(GOLANGCI_LINT_MODULES) run ./...

actionlint: $(ACTIONLINT) $(SHELLCHECK) ## Check GitHub workflows with actionlint (incl. shellcheck on run blocks)
	@echo "==> Checking workflows with actionlint..."
	@$(ACTIONLINT) -shellcheck=$(SHELLCHECK)

lint-fix: $(GOLANGCI_LINT_MODULES) ## Fix source code with all golangci linters
	@echo "==> Checking source code against linters (applying autofixes)..."
	$(GOLANGCI_LINT_MODULES) run --fix ./...

yamllint: $(YAMLLINT) ## Check YAML files with yamllint (config in .yamllint.yml)
	@echo "==> Checking YAML files with yamllint..."
	@$(YAMLLINT) -s .

shellcheck: $(SHELLCHECK) ## Check shell scripts with shellcheck
	@echo "==> Checking shell scripts with shellcheck..."
	@$(SHELLCHECK) .scripts/*.sh

zizmor: $(ZIZMOR) ## Audit GitHub workflows for security issues with zizmor
	@echo "==> Auditing workflows with zizmor..."
	@$(ZIZMOR) .

gencheck: generate ## Check that the definitions and generated SDKs match the specs (regenerate, then diff)
	@test -z "$$(git status --porcelain -- api-defs sdk/emby sdk/jf sdk/tmdb)" || \
		(git status --short -- api-defs sdk/emby sdk/jf sdk/tmdb; echo; \
		echo "api-defs/, sdk/emby, sdk/jf or sdk/tmdb is stale. Run 'make generate' and commit."; exit 1)

apicheck: ## Check that the definitions match the specs and every operation has a method in sdk/emby, sdk/jf and sdk/tmdb
	@echo "==> Checking API coverage of sdk/emby, sdk/jf and sdk/tmdb..."
	@go run ./sdk check -quiet

typos: $(TYPOS) ## Check spelling in code and docs with typos (config in .typos.toml)
	@echo "==> Checking spelling with typos..."
	@$(TYPOS) || \
		(echo; echo "Spelling errors found. Fix them with 'make typos-fix', or add false positives (names, API words) to .typos.toml."; exit 1)

typos-fix: $(TYPOS) ## Fix spelling errors found by typos
	@$(TYPOS) --write-changes

markdownlint: $(MARKDOWNLINT) ## Check the repo's markdown with markdownlint (config in .markdownlint.yml)
	@echo "==> Checking markdown with markdownlint..."
	@$(MARKDOWNLINT) $(MARKDOWN_INPUTS)

depscheck: ## Check that go.mod/go.sum and vendor/ are in sync
	@echo "==> Checking source code with go mod tidy..."
	@go mod tidy
	@git diff --exit-code -- go.mod go.sum || \
		(echo; echo "Unexpected difference in go.mod/go.sum files. Run 'go mod tidy' command or revert any go.mod/go.sum changes and commit."; exit 1)
	@echo "==> Checking source code with go mod vendor..."
	@go mod vendor
	@git diff --compact-summary --exit-code -- vendor || \
		(echo; echo "Unexpected difference in vendor/ directory. Run 'go mod vendor' command or revert any go.mod/go.sum/vendor changes and commit."; exit 1)
	@echo "==> Checking .tools/go.mod with go mod tidy..."
	@cd .tools && go mod tidy
	@git diff --exit-code -- .tools/go.mod .tools/go.sum || \
		(echo; echo "Unexpected difference in .tools/go.mod/go.sum. Run 'cd .tools && go mod tidy' and commit."; exit 1)
	@echo "==> Checking .tools/.custom-gcl.yml golangci-lint version matches .tools/go.mod..."
	@modv=$$(cd .tools && go list -m -f '{{.Version}}' github.com/golangci/golangci-lint/v2); \
		gclv=$$(grep '^version:' .tools/.custom-gcl.yml | awk '{print $$2}'); \
		[ "$$modv" = "$$gclv" ] || \
		(echo; echo "golangci-lint version mismatch: .tools/go.mod has $$modv but .tools/.custom-gcl.yml has $$gclv - update .custom-gcl.yml to match."; exit 1)

##@ Testing
test: build ## Run tests
	go test ./... -timeout ${TEST_TIMEOUT}

# the live suites take the backend from EMBYFIN_TEST_BACKEND (emby or jellyfin), and every
# target below runs for both unless BACKENDS narrows it: make testacc BACKENDS=jellyfin
BACKENDS?=emby jellyfin

# SDKDIR is the folder of a backend's SDK suite under acceptance/
SDKDIR=$(if $(filter emby,$(1)),emby,jf)

test-sdk: ## Run the SDK tests against an already-running server (acceptance/emby or acceptance/jf), and the TMDB sweep (acceptance/tmdb, recorded)
	@[ -n "${EMBYFIN_SERVER}" ] && [ -n "${EMBYFIN_TOKEN}" ] && [ -n "${EMBYFIN_BACKEND}" ] || \
		(echo 'EMBYFIN_BACKEND, EMBYFIN_SERVER and EMBYFIN_TOKEN must be set; or use "make testacc"'; exit 1)
	go test -tags acceptance -count=1 ./acceptance/$(call SDKDIR,${EMBYFIN_BACKEND})/ ./acceptance/tmdb/ -timeout ${TEST_TIMEOUT} -v

test-embyfin: ## Run the sdk/embyfin tests against an already-running server
	@[ -n "${EMBYFIN_SERVER}" ] && [ -n "${EMBYFIN_TOKEN}" ] && [ -n "${EMBYFIN_BACKEND}" ] || \
		(echo 'EMBYFIN_BACKEND, EMBYFIN_SERVER and EMBYFIN_TOKEN must be set; or use "make testacc"'; exit 1)
	go test -tags acceptance -count=1 ./acceptance/embyfin/ -timeout ${TEST_TIMEOUT} -v

test-tools: ## Run the tool tests (behaviour, audits, providers) against an already-running server
	@[ -n "${EMBYFIN_SERVER}" ] && [ -n "${EMBYFIN_TOKEN}" ] && [ -n "${EMBYFIN_BACKEND}" ] || \
		(echo 'EMBYFIN_BACKEND, EMBYFIN_SERVER and EMBYFIN_TOKEN must be set; or use "make testacc"'; exit 1)
	go test -tags acceptance -count=1 ./acceptance/tools/ -timeout ${TEST_TIMEOUT} -v

# every suite under acceptance/ runs against a real server, and each gets its own container per
# backend: the SDK and sdk/embyfin tests create and delete libraries of their own, which would
# trample the tool suite's fixtures. The SDK containers sit on the tools port + 100 and the
# sdk/embyfin ones on + 200, so all three can run side by side.
# The tool suite also gets a second server, of the other kind and with nothing in it,
# for its test that two instances of embyfin-mcp serve two servers at once.
# live SUITE BACKEND [EXTRA ENV] [EXTRA ARGS] - start a container, run one suite (sdk, embyfin or tools), tear it down
define live
	@echo "==> $(1) on $(2)..."
	@set -e; \
		env=".testenv-$(1)-$(2).sh"; \
		export EMBYFIN_TEST_BACKEND=$(2) EMBYFIN_TEST_CONTAINER=embyfin-mcp-$(1)-$(2) \
			EMBYFIN_TEST_DATA=$${HOME}/.cache/embyfin-mcp/testenv/$(1)-$(2) $(3); \
		[ "$(1)" = "sdk" ] && export EMBYFIN_TEST_PORT=$$(( $$( [ $(2) = emby ] && echo 18097 || echo 18096 ) + 100 )) EMBYFIN_TEST_PROXY_PORT=18180 || true; \
		[ "$(1)" = "embyfin" ] && export EMBYFIN_TEST_PORT=$$(( $$( [ $(2) = emby ] && echo 18097 || echo 18096 ) + 200 )) EMBYFIN_TEST_PROXY_PORT=18190 || true; \
		beside=$$( [ $(2) = emby ] && echo jellyfin || echo emby ); \
		.scripts/testenv.sh up | grep '^export' > "$$env"; \
		trap 'st=$$?; [ $$st -eq 0 ] || .scripts/testenv.sh logs || true; \
			.scripts/testenv.sh down; [ "$(1)" != "tools" ] || EMBYFIN_TEST_BACKEND='"$$beside"' .scripts/testenv.sh beside-down; rm -f '"$$env"'; exit $$st' EXIT; \
		[ "$(1)" != "tools" ] || EMBYFIN_TEST_BACKEND=$$beside .scripts/testenv.sh beside | grep '^export' >> "$$env"; \
		. ./"$$env"; \
		go test -tags acceptance -count=1 $(if $(filter sdk,$(1)),./acceptance/$(call SDKDIR,$(2))/ ./acceptance/tmdb/,./acceptance/$(1)/) -timeout ${TEST_TIMEOUT} -v $(4)
endef

testacc-sdk: ## SDK tests in a throwaway container, for each backend
	@for b in $(BACKENDS); do $(MAKE) --no-print-directory testacc-sdk-$$b || exit 1; done

testacc-sdk-%:
	$(call live,sdk,$*)

testacc-embyfin: ## sdk/embyfin tests in a throwaway container, for each backend
	@for b in $(BACKENDS); do $(MAKE) --no-print-directory testacc-embyfin-$$b || exit 1; done

testacc-embyfin-%:
	$(call live,embyfin,$*)

testacc-tools: ## Tool tests in a throwaway container, for each backend
	@for b in $(BACKENDS); do $(MAKE) --no-print-directory testacc-tools-$$b || exit 1; done

testacc-tools-%:
	$(call live,tools,$*)

testacc: testacc-sdk testacc-embyfin testacc-tools ## Run every live suite for both backends, each in its own container

# TMDB is not a server the suites run: its sweep replays recorded answers and needs neither a
# container nor a token, and a recording needs a token and nothing else
test-tmdb: ## Sweep every TMDB GET against the recorded answers (no server, no token)
	go test -tags acceptance -count=1 -run 'TestTMDBSweep$$' ./acceptance/tmdb/ -timeout ${TEST_TIMEOUT} -v

record-tmdb: ## Record the TMDB sweep against the real API (needs EMBYFIN_TMDB_TOKEN)
	@[ -n "$${EMBYFIN_TMDB_TOKEN}$${EMBYFIN_TMDB_KEY}" ] || (echo 'EMBYFIN_TMDB_TOKEN must be set to record'; exit 1)
	EMBYFIN_TEST_RECORD=1 go test -tags acceptance -count=1 -run 'TestTMDBSweep$$' ./acceptance/tmdb/ -timeout ${TEST_TIMEOUT} -v

# Coverage has to span all three suites or it lies: the unit tests alone report
# a fraction for tools/, because almost everything real happens in the live
# suites behind the acceptance tag. Each writes binary coverage into its own
# directory and covdata merges them, which is stdlib tooling rather than a
# third-party merger.
COVERDIR?=.coverage
COVERPKG=./tools/...,./lib/...,./cli/...,./sdk/...
SDKS=/sdk/emby/[0-9]\|/sdk/jf/[0-9]\|/sdk/tmdb/[0-9]
# (the comma leads each backend's pair: foreach joins its results with spaces, which the
# recipes strip, so a trailing one would run two backends' directories together)
COVERDIRS=$(COVERDIR)/unit$(foreach b,$(BACKENDS),,$(COVERDIR)/sdk-$(b),$(COVERDIR)/embyfin-$(b),$(COVERDIR)/tools-$(b))

cover: ## Run every suite with coverage and report the total
	@rm -rf $(COVERDIR)
	@mkdir -p $(COVERDIR)/unit $(foreach b,$(BACKENDS),$(COVERDIR)/sdk-$(b) $(COVERDIR)/embyfin-$(b) $(COVERDIR)/tools-$(b))
	@echo "==> unit..."
	@go test -count=1 -coverpkg=$(COVERPKG) ./tools/ ./cli/ ./lib/... ./sdk/... \
		-args -test.gocoverdir=$(CURDIR)/$(COVERDIR)/unit >/dev/null
	@for b in $(BACKENDS); do \
		$(MAKE) --no-print-directory cover-sdk-$$b || exit 1; \
		$(MAKE) --no-print-directory cover-embyfin-$$b || exit 1; \
		$(MAKE) --no-print-directory cover-tools-$$b || exit 1; \
	done
	@go tool covdata textfmt -i=$(subst $(space),,$(COVERDIRS)) -o=$(COVERDIR)/coverage.all
	@# sdk/emby, sdk/jf and sdk/tmdb are generated, one mechanical method per operation (997 of
	@# them); the suites exercise the ones the tools rely on, so the total is for the
	@# hand-written code and the SDKs get a line of their own (the per-package
	@# figures below include them)
	@grep -v '$(SDKS)' $(COVERDIR)/coverage.all > $(COVERDIR)/coverage.out
	@(head -1 $(COVERDIR)/coverage.all; grep '$(SDKS)' $(COVERDIR)/coverage.all) > $(COVERDIR)/coverage-sdk.out
	@echo
	@go tool cover -func=$(COVERDIR)/coverage.out | tail -1
	@printf 'generated SDKs:\t\t\t\t\t\t(statements)\t%s\n' "$$(go tool cover -func=$(COVERDIR)/coverage-sdk.out | tail -1 | awk '{print $$NF}')"
	@echo "==> per package"
	@go tool covdata percent -i=$(subst $(space),,$(COVERDIRS))

cover-sdk-%:
	$(call live,sdk,$*,,-coverpkg=$(COVERPKG) -args -test.gocoverdir=$(CURDIR)/$(COVERDIR)/sdk-$*)

cover-embyfin-%:
	$(call live,embyfin,$*,,-coverpkg=$(COVERPKG) -args -test.gocoverdir=$(CURDIR)/$(COVERDIR)/embyfin-$*)

cover-tools-%:
	$(call live,tools,$*,,-coverpkg=$(COVERPKG) -args -test.gocoverdir=$(CURDIR)/$(COVERDIR)/tools-$*)

cover-html: cover ## Run every suite with coverage and open the HTML report
	@go tool cover -html=$(COVERDIR)/coverage.out

record: ## Re-record every provider cassette against the real TMDB/TheTVDB/OMDb, for each backend (needs EMBYFIN_TMDB_TOKEN; EMBYFIN_TEST_RECORD=1 on a testacc target records only what they lack)
	@for b in $(BACKENDS); do $(MAKE) --no-print-directory record-$$b || exit 1; done

# EMBYFIN_TEST_RECORD=all refreshes every recording a run touches;
# EMBYFIN_TEST_RECORD=1 records only the requests no cassette holds
record-%:
	@echo "==> re-recording against the real providers (this hits the network)..."
	$(call live,tools,$*,EMBYFIN_TEST_RECORD=all)
	$(call live,sdk,$*,EMBYFIN_TEST_RECORD=all)

record-check: ## Check the provider cassettes still match the real APIs, without rewriting them
	@for b in $(BACKENDS); do \
		echo "==> verifying cassettes against the real providers (this hits the network)..."; \
		$(MAKE) --no-print-directory record-check-$$b || exit 1; \
	done

record-check-%:
	$(call live,tools,$*,EMBYFIN_TEST_VERIFY=1)

testenv-up: ## Start and seed a throwaway media server (EMBYFIN_TEST_BACKEND=emby|jellyfin)
	@.scripts/testenv.sh up

testenv-down: ## Remove the throwaway media server (EMBYFIN_TEST_BACKEND=emby|jellyfin)
	@.scripts/testenv.sh down

check-all: build test testacc lint actionlint yamllint shellcheck typos markdownlint depscheck gencheck apicheck ## Run build + tests (incl. live) + all linters + depscheck

.PHONY: default all help fmt goimports build docker generate pandorest-import pandorest-generate pandorest-diff lint lint-fix actionlint yamllint shellcheck zizmor typos typos-fix markdownlint gencheck apicheck depscheck check-all install tools test
