.PHONY: build bundle serve install test gen check fmt fmt-check vet clean

# build produces the release binaries: it bundles the SPA into web/dist and
# embeds it into thingd, so bin/thingd serves the whole app on its own. thingd
# always embeds web/dist, so web/dist must exist for any go build/test/vet — a
# committed web/dist/.gitkeep keeps the embed compiling before the first build.
build:
	cd web && npm install && npm run build
	go build -o bin/thing ./cmd/thing
	go build -o bin/thingd ./cmd/thingd

# web/dist holds the embedded SPA, rebuilt only when a web source is newer than
# it (make skips the recipe otherwise), so a Go-only change doesn't rebundle.
WEB_SRC := $(shell find web/src -type f 2>/dev/null) web/index.html web/package.json web/tsconfig.json web/build.mjs
web/dist/index.html: $(WEB_SRC)
	cd web && npm run build

# bundle assembles the dev binary air runs each iteration (.air.toml). air has a
# single build command and can't branch on which file changed, so bundle is
# incremental: refresh the embedded SPA only when a web source changed, then
# relink. `go build` is cached, so a Go-unchanged relink after a frontend edit is
# cheap.
bundle: web/dist/index.html
	go build -o ./tmp/thingd ./cmd/thingd

# serve runs the dev loop through air (.air.toml): it rebuilds the single
# embedded binary and restarts thingd on any Go or frontend change, so dev is the
# same one-binary, one-port app as prod. The browser reloads itself over SSE when
# the server restarts (see web/src/live.ts), so there is no dev server or proxy.
# Open http://localhost:$(PORT); Ctrl-C stops it. PORT and DIR=<path> override the
# port and data dir, e.g. `make serve PORT=4400 DIR=./demo`.
PORT ?= 4319
serve:
	@command -v air >/dev/null || { echo "air not found; install: go install github.com/air-verse/air@latest"; exit 1; }
	@[ -d web/node_modules ] || (cd web && npm install)
	air -- --port $(PORT) $(if $(DIR),--dir $(DIR))

# install builds and installs both binaries into the Go bin directory
# (`go env GOBIN`, else `$GOPATH/bin`). thingd embeds web/dist, so the SPA is
# bundled first — otherwise an installed thingd would serve the placeholder dist.
install:
	cd web && npm install && npm run build
	go install ./cmd/thing ./cmd/thingd

test:
	go test ./...

# Regenerate the web wire types from the shared JSON Schema (schema/tree.json) via
# quicktype; see scripts/gen.sh. `check` runs this and fails on any diff, so the
# schema and generated.ts never drift; the Go side is held to the same schema by
# internal/exporter's schema test.
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
# `make gen` produces no diff (guards against generated-code drift). vet and test
# compile the embed directly (web/dist/.gitkeep keeps that building), so no
# separate embed check is needed.
check: fmt-check vet test
	@$(MAKE) gen
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "make gen produced a diff or working tree is dirty:"; \
		git status --porcelain; exit 1; \
	fi

clean:
	rm -rf bin tmp
