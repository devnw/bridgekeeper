all: build tidy lint fmt test

#-------------------------------------------------------------------------
# Variables
# ------------------------------------------------------------------------
env=CGO_ENABLED=1
op= op run --env-file="./.env" -- 
SHELL := $(shell which bash)
fuzzsh=https://raw.githubusercontent.com/devnw/workflows/refs/heads/main/fuzz.sh

pre-commit: update upgrade tidy fmt lint build test

test: 
	CGO_ENABLED=1 go test -v -cover -failfast -race ./...

fuzz:
	curl -fsSL $(fuzzsh) | $(SHELL)

bench:
	go test -bench=. -benchmem ./...

test-all: test fuzz

fmt: 
	nixfmt flake.nix
	goimports -w .
	gofmt -s -w .

lint: 
	golangci-lint run

gomod2nix:
	gomod2nix generate

build: gomod2nix test
	$(env) go build ./...

release-dev:
	$(env) $(op) goreleaser release --clean --snapshot

upgrade:
	pre-commit autoupdate
	go get -u ./...

update:
	git submodule update --recursive

tidy: fmt
	go mod tidy

release: 
	if [ -z "$(tag)" ]; then echo "tag is required"; exit 1; fi
	git tag -a ${tag} -m "${tag}"
	git push origin ${tag}

clean: 
	rm -rf dist
	rm -rf coverage

#-------------------------------------------------------------------------
# CI targets
#-------------------------------------------------------------------------
build-ci: lint
	$(env) go build ./...

test-ci: build-ci 
	CGO_ENABLED=1 go test \
				-cover \
				-covermode=atomic \
				-coverprofile=coverage.txt \
				-failfast \
				-race ./...
	make fuzz FUZZ_TIME=10


bench-ci: test-ci
	go test -bench=. ./... | tee bench-output.txt


release-ci: bench-ci
	$(env) $(op) goreleaser release --clean

#-------------------------------------------------------------------------
# Force targets
#-------------------------------------------------------------------------

FORCE: 

#-------------------------------------------------------------------------
# Phony targets
#-------------------------------------------------------------------------

.PHONY: build test lint fuzz all clean FORCE
