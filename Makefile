.PHONY: build serve install test gen check fmt fmt-check vet clean

build:
	go build -o bin/thing ./cmd/thing
	go build -o bin/thingd ./cmd/thingd

# serve runs the dev loop: thingd is the single entry on :4319 (serving /api and
# /events and reverse-proxying everything else to Vite for HMR), with Vite on
# :5173 behind it. Open http://localhost:4319; Ctrl-C stops both. Pass DIR=<path>
# to pick the data dir. (Go changes need a manual restart until air lands.)
serve:
	@[ -d web/node_modules ] || (cd web && npm install)
	@echo "→ open http://localhost:4319 (thingd single entry; Vite HMR behind it)"
	@trap 'kill 0' INT TERM EXIT; \
		go run ./cmd/thingd --port 4319 --dev http://localhost:5173 $(if $(DIR),--dir $(DIR)) & \
		npm --prefix web run dev & \
		wait

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
