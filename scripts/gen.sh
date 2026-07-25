#!/usr/bin/env bash
# Regenerate the web wire types from the shared JSON Schema. schema/tree.json is
# the single source of truth for thingd's /api/tree shape; quicktype turns it into
# TypeScript. Run via `make gen`; `make check` fails if the committed output drifts
# from a fresh run, so the schema and generated.ts stay in lockstep. The Go side is
# held to the same schema by internal/exporter's schema test.
set -euo pipefail

cd "$(dirname "$0")/.."

schema="schema/tree.json"
out="web/src/domain/generated.ts"
quicktype="web/node_modules/.bin/quicktype"

if [ ! -x "$quicktype" ]; then
	echo "gen: installing web deps (quicktype)..." >&2
	(cd web && npm install --no-audit --no-fund) >&2
fi
if [ ! -x "$quicktype" ]; then
	echo "gen: quicktype still missing after npm install" >&2
	exit 1
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

{
	echo "// Code generated from schema/tree.json by quicktype; DO NOT EDIT."
	echo "// Regenerate with \`make gen\` after changing the schema."
	echo
	"$quicktype" --src-lang schema --lang ts --just-types --top-level Node "$schema"
} >"$tmp"

mv "$tmp" "$out"
trap - EXIT
echo "gen: wrote $out from $schema"
