.PHONY: build serve install test gen check fmt fmt-check vet clean

build:
	go build -o bin/thing ./cmd/thing

# serve runs the web UI dev server (Vite on :5173). The frontend lives in web/.
# It is a scaffold for now: the tree/detail UI and its thingd backend land in
# later commits, so this serves the app shell only, with no API to talk to yet.
serve:
	@[ -d web/node_modules ] || (cd web && npm install)
	npm --prefix web run dev

# install builds and installs the binaries into the Go bin directory
# (`go env GOBIN`, else `$GOPATH/bin`).
install:
	go install ./cmd/thing

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
