.PHONY: build test gen check fmt fmt-check vet clean

build:
	go build -o bin/thing ./cmd/thing

test:
	go test ./...

# Regenerate code from the shared JSON Schema (schema/*.json).
# Wired to quicktype in a later commit; a no-op until schema/ exists.
gen:
	@if [ -d schema ]; then ./scripts/gen.sh; else echo "no schema/ yet; skipping gen"; fi

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@out="$$(gofmt -l $(shell find . -name '*.go' -not -path './vendor/*'))"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

# check verifies the tree is clean: formatting, vet, tests, and that
# `make gen` produces no diff (guards against generated-code drift).
check: fmt-check vet test
	@$(MAKE) gen
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "make gen produced a diff or working tree is dirty:"; \
		git status --porcelain; exit 1; \
	fi

clean:
	rm -rf bin
