all: build tidy lint fmt test

#-------------------------------------------------------------------------
# Variables
# ------------------------------------------------------------------------
env=CGO_ENABLED=1
SHELL := $(shell which bash)
FUZZ_TIME ?= 30

deps:
	mkdir -p out

pre-commit: deps upgrade tidy fmt lint build test

test:
	CGO_ENABLED=1 go test -v -cover -failfast -race ./...

fuzz:
	@for f in $$(grep -rn 'func Fuzz' --include='*_test.go' -l .); do \
		pkg=$$(dirname $$f); \
		for fn in $$(grep -oP 'func \KFuzz\w+' $$f); do \
			echo "fuzzing $$pkg::$$fn for $(FUZZ_TIME)s"; \
			go test -fuzz=$$fn -fuzztime=$(FUZZ_TIME)s $$pkg || true; \
		done; \
	done

bench:
	go test -bench=. -benchmem ./...

test-all: test fuzz

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

build: test
	$(env) go build ./...

release-dev:
	$(env) goreleaser release --clean --snapshot

upgrade:
	go get -u ./...

tidy: fmt
	go mod tidy

release:
	if [ -z "$(tag)" ]; then echo "tag is required"; exit 1; fi
	git tag -a "$(tag)" -m "$(tag)"
	git push origin "$(tag)"

clean:
	rm -rf dist
	rm -rf out

#-------------------------------------------------------------------------
# CI targets
#-------------------------------------------------------------------------
build-ci: lint
	$(env) go build ./...

test-ci: deps build-ci
	CGO_ENABLED=1 go test \
				-cover \
				-covermode=atomic \
				-coverprofile=./out/coverage.txt \
				-failfast \
				-race ./...
	make fuzz FUZZ_TIME=10


bench-ci: deps test-ci
	go test -bench=. ./... | tee ./out/bench-output.txt


release-ci: bench-ci
	$(env) goreleaser release --clean
