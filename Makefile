.PHONY: build serve install test gen check fmt fmt-check vet clean

build:
	go build -o bin/thing ./cmd/thing
	go build -o bin/thingd ./cmd/thingd

# serve runs the dev loop: the Vite dev server holds THINGD_WEB_PORT (default
# 4319, HMR) and proxies /api and /events to thingd's API on THINGD_API_PORT
# (default 4320). So the dev URL is the same http://localhost:4319 as prod.
# Override either on a collision, e.g. `make serve THINGD_WEB_PORT=4400
# THINGD_API_PORT=4401`. Ctrl-C stops both; DIR=<path> picks the data dir. (Go
# changes need a manual restart until air lands.)
THINGD_WEB_PORT ?= 4319
THINGD_API_PORT ?= 4320
serve:
	@[ -d web/node_modules ] || (cd web && npm install)
	@trap 'kill 0' INT TERM EXIT; \
		go run ./cmd/thingd --port $(THINGD_API_PORT) $(if $(DIR),--dir $(DIR)) & \
		THINGD_WEB_PORT=$(THINGD_WEB_PORT) THINGD_API_PORT=$(THINGD_API_PORT) npm --prefix web run dev & \
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
